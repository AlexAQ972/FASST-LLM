from .Reasoning import Reasoning
from .Action_mcp import Action_class
import asyncio
import logging

FORMAT_PROMPT = '''
Note that the output format is
<ST> ... </ST>: the current Scanning Tree structure
<Task> ... </Task>: if you need a clarification based on documentation
'''

class ReasoningModule():
    def __init__(self, api_key, url, model, service, docs, logger:logging.Logger, 
                 openai_base_url, max_reasoning_iterations = 10,
                 max_tool_iterations = 5, default_top_k = 5
                 ):
        self.api_key = api_key
        self.url = url
        self.model = model
        self.service = service
        self.docs = docs
        self.max_reasoning_iterations = max_reasoning_iterations

        self.logger = logger

        self.reasoning_llm = Reasoning(api_key=api_key, url=url)
        self.action_llm = Action_class(
            api_key=api_key, doc_paths=docs, model=model, 
            logger=logger, openai_base_url=openai_base_url, 
            max_tool_iterations=max_tool_iterations, 
            default_top_k=default_top_k
        )

    async def initialize(self):
        await self.action_llm.action_initialize()
        self.logger.info("Action Initialized")

    async def reasoning(self, prompt, requirements = None, ST_file = None):
        user_prompt = prompt
        user_prompt = user_prompt.replace('{service}', self.service)

        if requirements is not None:
            user_prompt = user_prompt.replace('No requirements', requirements)

        cnt = self.max_reasoning_iterations
        while True:
            flag, result = self.reasoning_llm.sendMessage(message=user_prompt, model=self.model)
            if flag is False:
                self.logger.warning('When Reasoning ' + result)
                return None

            self.logger.info(f'Reasoning result {result}')

            cnt -= 1
            if cnt == 0:
                self.logger.info('Max reasoning iterations')
                break

            flag, Task = self.reasoning_llm.get_Task()
            if not flag:
                self.logger.info('No task')
                break

            answer = await self.action_llm.get_answer(problem=Task)
            self.logger.info(f'Action result {answer}')

            user_prompt = answer + FORMAT_PROMPT

        ST = self.reasoning_llm.get_ST()
        if ST_file is None:
            return ST
        else:
            with open(ST_file, 'w+') as f:
                f.write(ST)
            return ST
        
    async def stop(self):
        await self.action_llm.close()

