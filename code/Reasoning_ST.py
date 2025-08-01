import asyncio
import logging

from Reasoning import ReasoningModule

#======================Parameters============================

# Chat parameters
api_key = "Your API key"
url = "Chat URL like https://openrouter.ai/api/v1/chat/completions"
openai_base_url = 'OpenAI Base URL'
model = 'Model name'

requirements = '''
User requirements
'''

# Service parameters
service = 'Service name'
docs = ["Path 1", "Path 2, ..."] # Documents path list
max_reasoning_iterations = 10

# File parameters
reasoning_prompt_file = 'Reasoning Prompt file path'
with open(reasoning_prompt_file, 'r') as f:
    reasoning_prompt = f.read()

ST_file = 'The Scanning Tree target file path'

# Log parameters
logging.basicConfig(filename='log file path', level=logging.INFO, format="%(asctime)s - %(name)s - %(levelname)s - %(message)s")
logger = logging.getLogger()
#============================================================

async def main():
    reasoning = ReasoningModule(
        api_key=api_key, url=url, model=model, service=service,
        docs=docs, logger=logger, max_reasoning_iterations=max_reasoning_iterations, 
        openai_base_url=openai_base_url
    )
    
    await reasoning.initialize()

    ST = await reasoning.reasoning(
        prompt=reasoning_prompt,
        requirements=requirements,
        ST_file=ST_file
    )

    await reasoning.stop()

if __name__ == '__main__':
    asyncio.run(main())
