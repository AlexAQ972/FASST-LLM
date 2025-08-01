from Chat import Chat

DEFAULT_SYSTEM_PROMPT = '''
You are an experienced network service Golang developer.
Your code should use the following format:
    <Code> Your code </Code>
'''

class Code_generation(Chat):
    def __init__(self, api_key, url, service_name, model, system_prompt=None, title='title_A', example_service='FTP',example_code=None):
        if system_prompt is None:
            system_prompt = DEFAULT_SYSTEM_PROMPT
        super().__init__(api_key, url, system_prompt, title)

        self.model = model
        self.service_name = service_name
        self.example_service = example_service
        self.example_code = example_code

    def generation(self, prompt, ST, logger=None, para_prompt = ''):
        user_prompt = prompt.replace('{example_service}', self.example_service)
        user_prompt = user_prompt.replace('{{example_code}}', self.example_code)
        user_prompt = user_prompt.replace('{service}', self.service_name)
        user_prompt = user_prompt.replace('{ST}', ST) + para_prompt
        flag, result = self.sendMessage(message=user_prompt, model=self.model)
        if flag is False:
            return False, result
        
        if logger is not None:
            logger.info(result)
        
        start = result.find('<Code>') + len('<Code>')
        end = result.find('</Code>')

        if end == -1:
            return False, result

        return True, result[start:end]
