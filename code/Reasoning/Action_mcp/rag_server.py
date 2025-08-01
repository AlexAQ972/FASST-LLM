import asyncio
import logging
import os
import argparse
import hashlib
import json

from typing import List, Optional
from logging.handlers import RotatingFileHandler

from llama_index.core import VectorStoreIndex, SimpleDirectoryReader, Settings, StorageContext, load_index_from_storage
from llama_index.embeddings.huggingface import HuggingFaceEmbedding
from llama_index.core.node_parser import SentenceSplitter
from llama_index.core.schema import NodeWithScore
from llama_index.readers.file import PyMuPDFReader

from mcp.server.fastmcp import FastMCP
from mcp.server import InitializationOptions, NotificationOptions
from mcp.server.stdio import stdio_server

# --- Global Variables ---
mcp_instance: Optional[FastMCP] = None
rag_index: Optional[VectorStoreIndex] = None
CFG_DEFAULT_TOP_K: int = 10

logging.basicConfig(filename='Server.log', level=logging.INFO, format="%(asctime)s - %(name)s - %(levelname)s - %(message)s")
logger = logging.getLogger("Test")

def _calculate_file_hash(filepath):
    """Calculates the SHA256 hash of a file."""
    sha256_hash = hashlib.sha256()
    with open(filepath, "rb") as f:
        for byte_block in iter(lambda: f.read(4096), b""):
            sha256_hash.update(byte_block)
    return sha256_hash.hexdigest()

def _generate_cache_signature(doc_files: List[str], embed_model: str, chunk_size: int, chunk_overlap: int) -> str:
    """Generates a unique signature based on docs, their content, and settings."""
    sorted_doc_files = sorted([os.path.abspath(p) for p in doc_files])
    
    file_hashes = {}
    for f_path in sorted_doc_files:
        if os.path.exists(f_path):
            file_hashes[f_path] = _calculate_file_hash(f_path)
    
    signature_data = {
        "doc_files": sorted_doc_files,
        "file_hashes": file_hashes,
        "embed_model": embed_model,
        "chunk_size": chunk_size,
        "chunk_overlap": chunk_overlap,
    }
    
    signature_str = json.dumps(signature_data, sort_keys=True)
    
    return hashlib.sha256(signature_str.encode('utf-8')).hexdigest()


def initialize_rag_resources(doc_files: List[str], embed_model: str, embed_device: str, chunk_size: int, chunk_overlap: int, persist_dir: str):
    """
    Initializes RAG resources. It validates the cache using a signature.
    If the cache is valid, it loads the index. Otherwise, it rebuilds and saves it.
    """
    global rag_index
    logger.info("Initializing RAG resources...")

    if not doc_files:
        logger.error("FATAL: No document files provided to build the RAG index.")
        raise ValueError("No document files specified for RAG.")

    current_signature = _generate_cache_signature(doc_files, embed_model, chunk_size, chunk_overlap)
    metadata_path = os.path.join(persist_dir, 'cache_metadata.json')
    
    is_cache_valid = False
    if os.path.exists(metadata_path):
        try:
            with open(metadata_path, 'r') as f:
                metadata = json.load(f)
                saved_signature = metadata.get('signature')
            
            if saved_signature == current_signature:
                logger.info("Cache signature matches. Attempting to load index from storage.")
                is_cache_valid = True
            else:
                logger.info("Cache signature mismatch. Rebuilding index. Reason: Configuration or document content changed.")
        except (FileNotFoundError, json.JSONDecodeError, KeyError):
            logger.warning("Could not read or validate cache metadata. Rebuilding index.")

    if is_cache_valid:
        try:
            logger.info(f"Loading index from cache at '{persist_dir}'...")
            Settings.embed_model = HuggingFaceEmbedding(model_name=embed_model, device=embed_device)
            storage_context = StorageContext.from_defaults(persist_dir=persist_dir)
            rag_index = load_index_from_storage(storage_context)
            logger.info("RAG Vector store index loaded successfully from cache.")
            return
        except Exception as e:
            logger.warning(f"Failed to load index from '{persist_dir}' despite valid signature: {e}. Rebuilding from scratch.")
            is_cache_valid = False

    logger.info("Building index from source documents...")
    
    Settings.embed_model = HuggingFaceEmbedding(model_name=embed_model, device=embed_device)
    sentence_splitter = SentenceSplitter(chunk_size=chunk_size, chunk_overlap=chunk_overlap)

    logger.info(f"Loading documents from: {doc_files}")
    documents = []
    for f_path in doc_files:
        if not os.path.exists(f_path):
            raise FileNotFoundError(f"Document file not found: {f_path}")
        
        try:
            documents.extend(PyMuPDFReader().load_data(file_path=f_path))
        except Exception as e:
            logger.warning(f"Failed to read {f_path} with PyMuPDFReader ({e}), falling back to SimpleDirectoryReader.")
            documents.extend(SimpleDirectoryReader(input_files=[f_path]).load_data())
    
    if not documents:
        raise ValueError("No documents were loaded for RAG.")

    logger.info("Building vector store index from documents...")
    rag_index = VectorStoreIndex.from_documents(documents, transformations=[sentence_splitter])
    
    logger.info(f"Persisting index to '{persist_dir}' for future use...")
    os.makedirs(persist_dir, exist_ok=True)
    rag_index.storage_context.persist(persist_dir=persist_dir)
    
    with open(metadata_path, 'w') as f:
        json.dump({'signature': current_signature}, f)
        
    logger.info("RAG Vector store index built and persisted successfully.")


def register_mcp_tools(mcp: FastMCP, default_top_k_for_tool: int):
    """Registers MCP tools."""
    global CFG_DEFAULT_TOP_K 
    CFG_DEFAULT_TOP_K = default_top_k_for_tool

    @mcp.tool()
    def retrieve_document_chunks(query: str, top_k: Optional[int] = None) -> List[str]:
        """
        Retrieves the most relevant text chunks from the knowledge base based on the user query.
        Use this tool when you need to find specific information or context from the provided service documents.

        Args:
          - query (str): The user's query string, describing the information to be found (required).
          - top_k (Optional[int]): The number of most relevant text chunks to return. Defaults to server configuration ({doc_placeholder_default_top_k}).

        Returns:
          - List[str]: A list of relevant text chunks. Returns an empty list if nothing is found.
        """
        global rag_index, CFG_DEFAULT_TOP_K, logger

        if rag_index is None:
            logger.error("RAG index is not initialized. Cannot retrieve.")
            raise RuntimeError("RAG index is not initialized. Tool call failed.")

        actual_top_k = top_k if top_k is not None and top_k > 0 else CFG_DEFAULT_TOP_K
        logger.info(f"RAG Tool: Received query='{query}', top_k={actual_top_k}")
        
        retriever_instance = rag_index.as_retriever(similarity_top_k=actual_top_k)
        retrieved_nodes: List[NodeWithScore] = retriever_instance.retrieve(query)
        
        if not retrieved_nodes:
            logger.info(f"RAG Tool: No relevant chunks found for query='{query}'")
            return []

        result_texts = [node.get_text() for node in retrieved_nodes]
        logger.info(f"RAG Tool: Returning {len(result_texts)} chunks for query='{query}'")
        return result_texts
    
    retrieve_document_chunks.__doc__ = retrieve_document_chunks.__doc__.format(
        doc_placeholder_default_top_k=default_top_k_for_tool
    )

async def run_server_main_logic(args):
    """Main logic for setting up and running the server after args are parsed."""
    global mcp_instance, logger

    try:
        initialize_rag_resources(
            doc_files=args.docs,
            embed_model=args.embed_model,
            embed_device=args.embed_device,
            chunk_size=args.chunk_size,
            chunk_overlap=args.chunk_overlap,
            persist_dir=args.persist_dir
        )
    except Exception as e:
        logger.critical(f"Failed to initialize RAG resources for '{args.server_name}': {e}. Server exiting.")
        return

    mcp_instance = FastMCP(args.server_name)
    register_mcp_tools(mcp_instance, args.default_top_k)

    async with stdio_server() as (read_stream, write_stream):
        init_options = InitializationOptions(
            server_name=args.server_name,
            server_version="1.3.0",
            capabilities=mcp_instance._mcp_server.get_capabilities(
                notification_options=NotificationOptions(),
                experimental_capabilities={}
            )
        )
        logger.info(f"Starting MCP Server '{args.server_name}' via STDIO...")
        await mcp_instance._mcp_server.run(read_stream, write_stream, init_options)

def parse_arguments():
    parser = argparse.ArgumentParser(description="Run RAG MCP Server Process.")
    parser.add_argument("--docs", nargs='+', required=True, help="List of document file paths for RAG.")
    parser.add_argument("--embed-model", default="BAAI/bge-m3", help="Embedding model name.")
    parser.add_argument("--embed-device", default="cpu", help="Device for embedding model (cpu or cuda).")
    parser.add_argument("--chunk-size", type=int, default=1024, help="Chunk size for document splitting.")
    parser.add_argument("--chunk-overlap", type=int, default=128, help="Chunk overlap for document splitting.")
    parser.add_argument("--default-top-k", type=int, default=3, help="Default K for similarity search for RAG tool.")
    parser.add_argument("--server-name", default="rag-stdio-server", help="Name for this MCP server instance.")
    parser.add_argument("--log-file", type=str, default=None, help="Path to the log file. If None, logs to console + default file if any.")
    parser.add_argument("--log-level", type=str, default="INFO", choices=["DEBUG", "INFO", "WARNING", "ERROR", "CRITICAL"], help="Logging level.")
    parser.add_argument("--persist-dir", default="./storage_cache", help="Directory to store and load the RAG index cache.")
    return parser.parse_args()

if __name__ == "__main__":
    cli_args = parse_arguments()
    try:
        asyncio.run(run_server_main_logic(cli_args))
    except Exception as e:
        if logger.handlers:
             logger.critical(f"Unhandled exception in __main__: {e}", exc_info=True)
        else:
            print(f"CRITICAL UNHANDLED EXCEPTION in __main__: {e}")
            import traceback
            traceback.print_exc()