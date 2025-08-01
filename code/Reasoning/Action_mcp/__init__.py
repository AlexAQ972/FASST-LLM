from .client import ACTIONLLM

from pathlib import Path

import logging


SERVER_SCRIPT_PATH = Path(__file__).parent / 'rag_server.py'

class Action_class(ACTIONLLM):
    def __init__(self, api_key, doc_paths, model, openai_base_url, logger = None, max_tool_iterations = 5, default_top_k = 5):
        
        server_configurations = [
            {
                "id": "rag-stdio-server",
                "type": "rag",
                "script_path": str(SERVER_SCRIPT_PATH),
                "rag_docs": doc_paths,
                "rag_default_top_k": default_top_k,
            }
        ]
        super().__init__(server_configurations, api_key, openai_base_url, model, max_tool_iterations, logger=logger)

    async def action_initialize(self):
        try:
            await self.initialize()
            if not self.available_openai_tools:
                print("Warning: No tools are available. LLM will not be able to query documents.")
        except Exception as e:
            print(f"An error occurred: {e}")
        return

    async def get_answer(self, problem):
        final_answer, updated_history = await self.process_task(
            user_task_description=problem,
        )
        return final_answer
