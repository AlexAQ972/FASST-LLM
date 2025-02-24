from llama_index.core import VectorStoreIndex, SimpleDirectoryReader, Settings, StorageContext, load_index_from_storage
from llama_index.embeddings.huggingface import HuggingFaceEmbedding
from llama_index.core.node_parser import SentenceSplitter
from llama_index.llms.openrouter import OpenRouter
from llama_index.core.retrievers import VectorIndexRetriever
from llama_index.core import get_response_synthesizer


# Read the service specification files
documents = SimpleDirectoryReader(input_files=['your file here']).load_data()

# Assign models
Settings.embed_model = HuggingFaceEmbedding(model_name="BAAI/bge-m3", device='cpu')
Settings.llm = OpenRouter(
    api_key = "",
    temperature=0.3,
    model="openai/gpt-4o-2024-11-20"
)

sentence_spliter = SentenceSplitter(chunk_size=1024, chunk_overlap=128)

# Get embeddings
index = VectorStoreIndex.from_documents(
    documents, transformations=[sentence_spliter]
)


# retrieve most related chunks
retriever = VectorIndexRetriever(index=index, similarity_top_k=10)

response = []
response.extend(retriever.retrieve(str_or_query_bundle="establishing a connection or completing the handshake."))
response.extend(retriever.retrieve(str_or_query_bundle="Get server information or get server capabilities."))

response = sorted(response, key=lambda x:x.score, reverse=True)


# synthetize chunks
synthesizer = get_response_synthesizer(response_mode="simple_summarize")
syn_res = synthesizer.synthesize("""I need to implement a plugin for <service name>.
                                 Please rephrase the document parts related to establishing the connection, 
                                 completing the handshake and acquiring server information. You need to pay
                                 special attention to how the packets are created and what's the meaning of each 
                                 fields of requests and responses so that any experienced programmer can make
                                 valid request according to your answers only.""",
                              nodes=response)