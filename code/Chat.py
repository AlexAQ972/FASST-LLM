import requests
import json

class Chat:
    def __init__(self, api_key, url, system_prompt=None, title:str='title_A'):
        if system_prompt is None:
            system_prompt = 'You are an AI assistant that helps people find information.'
        self.messages = [{"role": "system", "content": system_prompt}]
        self.request_header = {
            "Authorization": f'Bearer {api_key}',
            "X-Title": f'{title}',
            "Content-Type": "application/json"
        }
        self.url = url

    def sendMessage(self, message, model, temperature:float=0.3):
        self.messages.append({"role": "user", "content":message})
        request_message = {
            "model": model,
            "messages": self.messages,
            "temperature": temperature
        }
        response = requests.post(
            url=self.url,
            json=request_message,
            headers=self.request_header
        )
        json_result = json.loads(response.content.decode('utf-8'))
        try:
            content = json_result['choices'][0]['message']['content']
            self.messages.append({"role": "assistant", "content": content})
            return True, content
        except KeyError:
            error = json_result['error']['message']
            self.messages.pop()
            return False, error