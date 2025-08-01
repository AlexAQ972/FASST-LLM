from Propagation import Loss
from Propagation import Grad
from Propagation import Optimizer

import logging

#======================Parameters============================

# Chat parameters
api_key = "Your API key"
url = "Chat URL like https://openrouter.ai/api/v1/chat/completions"
model = 'Model name'

client_log = '''
'''

server_log = '''
'''

# File parameters
ST_file = 'ST'
with open(ST_file, 'r') as f:
    ST = f.read()

code_file = 'scanner.go'
with open(code_file, 'r') as f:
    code = f.read()

reasoning_prompt_file = 'reasoning_prompt.txt'
with open(reasoning_prompt_file, 'r') as f:
    reasoning_prompt = f.read()

generation_prompt_file = 'generation_prompt.txt'
with open(generation_prompt_file, 'r') as f:
    generation_prompt = f.read()

# Log parameters
logging.basicConfig(filename='log file path', level=logging.INFO, format="%(asctime)s - %(name)s - %(levelname)s - %(message)s")
logger = logging.getLogger()

#============================================================

logger.info('Begin')

loss_fn = Loss(api_key=api_key, url=url, model=model)
grad_fn = Grad(api_key=api_key, url=url, model=model)
optimizer = Optimizer(api_key=api_key, url=url, model=model)

logger.info('Start Calculation')

flag, loss = loss_fn.get_loss(client_log=client_log, server_log=server_log)
if not flag:
    print("No loss")
    logger.info(f'Result is {loss}')
    exit(0)
logger.info(f'Get loss {loss}')

flag, code_grad = grad_fn.get_code_grad(code=code, loss=loss)
if not flag:
    print("No code grad")
    logger.info(f'Result is {code_grad}')
    exit(0)
logger.info(f'Get code grad {code_grad}')

flag, generation_prompt_grad = grad_fn.get_prompt_grad(prompt=generation_prompt, grad=code_grad)
if not flag:
    print('No generation prompt grad')
    logger.info(f'Result is {generation_prompt_grad}')
    exit(0)
logger.info(f'Get generation prompt grad {generation_prompt_grad}')

flag, ST_grad = grad_fn.get_ST_grad(ST=ST, grad=code_grad)
if not flag:
    print("No ST grad")
    logger.info(f'Result is {ST_grad}')
    exit(0)
logger.info(f'Get ST grad {ST_grad}')

flag, reasoning_prompt_grad = grad_fn.get_prompt_grad(prompt=reasoning_prompt, grad=ST_grad)
if not flag:
    print('No reasoning prompt grad')
    logger.info(f'Result is {reasoning_prompt_grad}')
    exit(0)
logger.info(f'Get reasoning prompt grad {reasoning_prompt_grad}')

flag, new_reasoning_prompt = optimizer.step_prompt(prompt=reasoning_prompt, grad=reasoning_prompt_grad)
if not flag:
    print('No prompt')
    logger.info(f'Result is {new_reasoning_prompt}')
    exit(0)
logger.info(f'New reasoning prompt is {new_reasoning_prompt}')

flag, new_generation_prompt = optimizer.step_prompt(prompt=generation_prompt, grad=generation_prompt_grad)
if not flag:
    print('No prompt')
    logger.info(f'Result is {new_generation_prompt}')
    exit(0)
logger.info(f'New generation prompt is {new_generation_prompt}')

with open(reasoning_prompt_file, 'w') as f:
    f.write(new_reasoning_prompt)

with open(generation_prompt_file, 'w') as f:
    f.write(new_generation_prompt)
