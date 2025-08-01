from Generator.Code_generator import Code_generation

import logging

#======================Parameters============================

# Chat parameters
api_key = "Your API key"
url = "Chat URL like https://openrouter.ai/api/v1/chat/completions"
model = 'Model name'

para_prompt = f'''
Other Parameters:

'''

# Service parameters
service = 'Service name'
example_service = 'ftp'
example_code_file = 'ftp.go'
with open(example_code_file, 'r') as f:
    example_code = f.read()

# File parameters
generation_prompt_file = 'Generation Prompt file path'
with open(generation_prompt_file, 'r') as f:
    generation_prompt = f.read()

ST_file = 'The Scanning Tree file path'
code_file = 'The Code target file path'

with open(ST_file, 'r') as f:
    ST = f.read()

# Log parameters
logging.basicConfig(filename='log file path', level=logging.INFO, format="%(asctime)s - %(name)s - %(levelname)s - %(message)s")
logger = logging.getLogger()

#============================================================

code_generation = Code_generation(
    api_key=api_key, url=url, model=model,
    service_name=service,
    example_service=example_service, 
    example_code=example_code
)

logger.info(f'ST: {ST}')

flag, code = code_generation.generation(prompt=generation_prompt, ST=ST, logger=logger, para_prompt=para_prompt)

with open(code_file, 'w+') as f:
    f.write(code)


