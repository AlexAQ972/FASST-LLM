from Chat import Chat

api_key = "your api key"
url = "your api url"


model = 'model name in openrouter'

service_name = ""
coding_outline = ""

system_prompt = "You are an experienced network service Golang developer."

with open('ftp.go', 'r') as f:
    ftp_code = f.read()

user_prompt = f"""
Here are the example code of FTP scanning plugin of our tool. Please read them carefully first.
<code>
{ftp_code}
</code>

Now you need to implement the {service_name} scanning plugin according to example codes in Golang.
Our toolwill call the "Scan" function to start the scan process. As you may notice, there are some tool functions used in the example codes. When you're implementing the new plugin, please consider use the existing tool functions before you implement a new one of similar usage.
Your implementing steps are as follows:
<steps>
{coding_outline}
</steps>
"""

chat = Chat(api_key=api_key, url=url, system_prompt=system_prompt)

flag, result = chat.sendMessage(message=user_prompt, model=model)