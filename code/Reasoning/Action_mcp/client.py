import asyncio
import json
import os
import sys
import logging

from typing import List, Dict, Any, Optional, Tuple, Type # Added Type
from contextlib import AsyncExitStack

from mcp import ClientSession, StdioServerParameters
from mcp.client.stdio import stdio_client
from openai import AsyncOpenAI
from openai.types.chat import ChatCompletionMessageParam, ChatCompletionToolParam

DEFAULT_SYSTEM_PROMPT = '''
You are an expert AI assistant specializing in analyzing and executing tasks related to specific internet services. Your goal is to help users understand and complete given sub-tasks.

Your capabilities include:
1.  **Task Understanding and Rephrasing**: Clearly articulate the goal and scope of the sub-task.
2.  **Planning Execution Steps**: Break down complex sub-tasks into a logical and actionable sequence of steps.
3.  **Information Retrieval**: When you need specific information from the service documentation to plan or execute steps, you can use the provided `retrieve_document_chunks` tool (it will be prefixed with the server ID, e.g., `my_mqtt_docs_retrieve_document_chunks`). When calling this tool, provide a clear and specific query to get the most relevant document snippets. **Only use this tool when document information is genuinely needed to answer the question or formulate a plan.**
4.  **Result Generation**: Based on your planning and retrieved information (if applicable), provide a comprehensive, accurate, and easy-to-understand answer or solution.

Your workflow should be:
Step 1: Understand & Rephrase Task: First, carefully understand the user's sub-task. Rephrase the task in your own words to ensure you have accurately grasped its core requirements.
Step 2: Plan Execution: Then, devise a detailed step-by-step plan to accomplish this sub-task. List out what needs to be done at each step.
Step 3: Identify Information Gaps & Use Tools if Necessary: As you plan each step, consider if specific information from the service documentation is required. If so, clearly identify what information you need, formulate a targeted query, and then call the appropriate `retrieve_document_chunks` tool associated with the relevant document set.
Step 4: Synthesize Information & Generate Response: After gathering all necessary information (including document content retrieved via tools), synthesize this information and, following your plan, generate the final answer. The answer should be clear, accurate, and directly address the user's sub-task.

Ensure your responses are well-structured; if steps are involved, list them clearly. If information returned by a tool is insufficient or irrelevant, please state this and explain how you will proceed.

The final output should be the complete solution or explanation for the user's original sub-task, not your thought process itself (unless explicitly asked for).
'''

class ACTIONLLM:
    def __init__(
        self,
        server_configs: List[Dict[str, Any]], 
        openai_api_key: Optional[str] = None,
        openai_base_url: Optional[str] = None,
        llm_model_name: str = None,
        max_tool_iterations: int = None,
        logger = None
    ):
        self.llm_model_name = llm_model_name
        self.max_tool_iterations = max_tool_iterations
        self.server_configs = server_configs
        
        self.api_key = openai_api_key or os.getenv("OPENAI_API_KEY")
        self.base_url = openai_base_url or os.getenv("OPENAI_BASE_URL")

        if not self.api_key:
            raise ValueError("OpenAI API Key is required.")

        self.client = AsyncOpenAI(base_url=self.base_url, api_key=self.api_key)
        
        self.sessions: Dict[str, Tuple[ClientSession, Any, Any]] = {}
        self.tool_mapping: Dict[str, Tuple[ClientSession, str]] = {}
        self.available_openai_tools: List[ChatCompletionToolParam] = []
        self._exit_stack = AsyncExitStack()
        self._is_initialized = False
        self.log_file = logger

    async def initialize(self):
        if self._is_initialized:
            self.log_file.info("Action llm already initialized.")
            return

        self.log_file.info("Initializing Action llm...")
        for config in self.server_configs:
            server_id = config.get("id")
            script_path = config.get("script_path")
            server_type = config.get("type", "generic")

            if not server_id or not script_path:
                self.log_file.error(f"Invalid server config, missing 'id' or 'script_path': {config}. Skipping.")
                continue
            
            if not os.path.exists(script_path):
                self.log_file.error(f"Script {script_path} for server '{server_id}' not found. Skipping.")
                continue

            # Construct command arguments
            cmd_args = [script_path] # First arg after 'python' is the script itself
            
            cmd_args.extend(["--server-name", server_id])

            if server_type == "rag":
                rag_docs = config.get("rag_docs")
                if not rag_docs or not isinstance(rag_docs, list):
                    self.log_file.error(f"RAG server '{server_id}' misconfigured: 'rag_docs' list is missing or invalid. Skipping.")
                    continue
                cmd_args.extend(["--docs"] + rag_docs) # Add document paths

                # Add other optional RAG parameters if provided in config
                if "rag_embed_model" in config: cmd_args.extend(["--embed-model", config["rag_embed_model"]])
                if "rag_embed_device" in config: cmd_args.extend(["--embed-device", config["rag_embed_device"]])
                if "rag_chunk_size" in config: cmd_args.extend(["--chunk-size", str(config["rag_chunk_size"])])
                if "rag_chunk_overlap" in config: cmd_args.extend(["--chunk-overlap", str(config["rag_chunk_overlap"])])
                if "rag_default_top_k" in config: cmd_args.extend(["--default-top-k", str(config["rag_default_top_k"])])
                if "log_file" in config: cmd_args.extend(["--log-file", str(config["log_file"])])

            params = StdioServerParameters(command=sys.executable, args=cmd_args, env=os.environ.copy())
            
            try:
                self.log_file.info(f"Starting MCP server: '{server_id}' from script: {script_path} with args: {' '.join(cmd_args)}")
                stdio_ctx = stdio_client(params)
                stdio = await self._exit_stack.enter_async_context(stdio_ctx)
                session_ctx = ClientSession(*stdio)
                session = await self._exit_stack.enter_async_context(session_ctx)
                
                init_response = await session.initialize() 
                self.log_file.info(f"  Initialization response from '{server_id}': {init_response}")
                self.sessions[server_id] = (session, session_ctx, stdio_ctx)

                tool_list_response = await session.list_tools()

                if not tool_list_response.tools:
                    self.log_file.warning(f"  Warning: No tools listed by server '{server_id}'.")
                
                for tool_def in tool_list_response.tools:
                    prefixed_tool_name = f"{server_id}_{tool_def.name}" # Use the unique server_id from config
                    self.tool_mapping[prefixed_tool_name] = (session, tool_def.name)
                    self.available_openai_tools.append({
                        "type": "function",
                        "function": {
                            "name": prefixed_tool_name,
                            "description": tool_def.description,
                            "parameters": tool_def.inputSchema,
                        }
                    })
                    self.log_file.info(f"  - Registered tool: {prefixed_tool_name} (from {tool_def.name} on '{server_id}')")
                self.log_file.info(f"  Successfully connected to server '{server_id}'.")

            except Exception as e:
                self.log_file.error(f"Failed to connect or initialize session with server '{server_id}' ({script_path}): {e}")
        
        if not self.tool_mapping:
            self.log_file.warning("Warning: No tools were successfully registered from any MCP server.")
        
        self._is_initialized = True
        self.log_file.info("Action llm initialized successfully.")

    async def process_task(
        self,
        user_task_description: str,
        conversation_history: Optional[List[ChatCompletionMessageParam]] = None
    ) -> Tuple[str, List[ChatCompletionMessageParam]]:
        if not self._is_initialized:
            raise RuntimeError("Action llm not initialized. Call initialize() first.")

        user_prompt = f'''
        Hello! I am trying to understand and implement a feature related to a specific service.
        My current sub-task is: "{user_task_description}"

        Please follow your defined workflow to help me with this sub-task.
        Begin your thought process now.
        '''

        if conversation_history is None:
            messages: List[ChatCompletionMessageParam] = [
                {"role": "system", "content": DEFAULT_SYSTEM_PROMPT},
                {"role": "user", "content": user_prompt}
            ]
        else:
            messages = conversation_history
            if not any(msg["role"] == "system" for msg in messages):
                messages.insert(0, {"role": "system", "content": DEFAULT_SYSTEM_PROMPT})
            messages.append({"role": "user", "content": user_prompt})


        tool_choice_param = "auto" if self.available_openai_tools else None 

        for iteration in range(self.max_tool_iterations):
            self.log_file.info(f"\n--- LLM Interaction (Iteration {iteration + 1}) ---")
            try:
                completion_params = {"model": self.llm_model_name, "messages": messages}
                if self.available_openai_tools:
                    completion_params["tools"] = self.available_openai_tools
                    completion_params["tool_choice"] = tool_choice_param
                
                response = await self.client.chat.completions.create(**completion_params)

            except Exception as e:
                error_message = f"Error calling OpenAI API: {e}"
                self.log_file.error(error_message)
                return error_message, messages 

            response_message = response.choices[0].message
            
            message_to_append = {"role": response_message.role}
            if response_message.content: message_to_append["content"] = response_message.content
            if response_message.tool_calls: message_to_append["tool_calls"] = [tc.model_dump(exclude_none=True) for tc in response_message.tool_calls]
            
            if message_to_append.get("content") or message_to_append.get("tool_calls"):
                messages.append(message_to_append)

            if not response_message.tool_calls:
                self.log_file.info("LLM provided a response without tool calls.")
                final_answer = response_message.content or "LLM provided no textual content."
                return final_answer, messages

            self.log_file.info(f"LLM requested {len(response_message.tool_calls)} tool call(s).")
            tool_messages_to_add: List[ChatCompletionMessageParam] = []
            # ... (Tool call processing logic from previous version - ensure it's copied accurately) ...
            for tool_call in response_message.tool_calls:
                tool_name_with_prefix = tool_call.function.name
                tool_call_id = tool_call.id

                if tool_name_with_prefix not in self.tool_mapping:
                    error_msg = f"Error: Tool '{tool_name_with_prefix}' not found in local mapping."
                    self.log_file.error(error_msg)
                    tool_messages_to_add.append({
                        "role": "tool", "tool_call_id": tool_call_id, 
                        "name": tool_name_with_prefix, "content": error_msg
                    })
                    continue

                mcp_session, original_tool_name = self.tool_mapping[tool_name_with_prefix]
                
                try:
                    tool_args_str = tool_call.function.arguments
                    # Defensive check for empty or non-JSON string arguments
                    if not tool_args_str or not tool_args_str.strip().startswith('{'):
                         tool_args = {}
                         if tool_args_str and tool_args_str.strip(): # Log if it was non-empty but not JSON
                            self.log_file.warning(f"Warning: Tool arguments for {tool_name_with_prefix} not a valid JSON object string: '{tool_args_str}'. Proceeding with empty args.")
                    else:
                        tool_args = json.loads(tool_args_str)

                    self.log_file.info(f"  Calling tool: {original_tool_name} (prefixed: {tool_name_with_prefix}) with args: {tool_args}")
                    
                    mcp_tool_response = await mcp_session.call_tool(original_tool_name, tool_args)
                    tool_result_content = str(mcp_tool_response.content) 
                    
                    self.log_file.info(f"  Tool '{original_tool_name}' returned (first 200 chars): {tool_result_content[:200]}...")
                    tool_messages_to_add.append({
                        "role": "tool", "tool_call_id": tool_call_id, 
                        "name": tool_name_with_prefix, "content": tool_result_content
                    })
                except json.JSONDecodeError as e:
                    error_msg = f"Error: Invalid JSON arguments for tool {tool_name_with_prefix}: {tool_call.function.arguments}. Details: {e}"
                    self.log_file.error(error_msg)
                    tool_messages_to_add.append({
                        "role": "tool", "tool_call_id": tool_call_id, 
                        "name": tool_name_with_prefix, "content": error_msg
                    })
                except Exception as e:
                    error_msg = f"Error calling tool {tool_name_with_prefix} via MCP: {e}"
                    self.log_file.error(error_msg)
                    tool_messages_to_add.append({
                        "role": "tool", "tool_call_id": tool_call_id, 
                        "name": tool_name_with_prefix, "content": error_msg
                    })
            
            messages.extend(tool_messages_to_add)

        self.log_file.info("Max tool iterations reached.")
        final_answer = "Maximum tool iterations reached. The LLM might not have fully completed the task."
        if messages and messages[-1]["role"] == "assistant" and messages[-1].get("content"):
            final_answer = messages[-1]["content"] + "\n\n"
            
        return final_answer, messages

    async def close(self):
        if self._is_initialized:
            self.log_file.info("Closing Action llm and releasing resources...")
            await self._exit_stack.aclose()
            self.sessions.clear()
            self.tool_mapping.clear()
            self.available_openai_tools.clear()
            self._is_initialized = False
            self.log_file.info("Action llm closed.")