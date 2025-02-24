from Chat import Chat

api_key = "your api key"
url = "your api url"


model = 'model name in openrouter'

service_name = ""
Document_extraction_output = ""

system_prompt = "You are an experienced network service Golang developer."

user_prompt = f"""
Here's part of document of {service_name}:
<doc>
{Document_extraction_output}
</doc>

Please follow the following instruction step by step to implement the <service name> scanning plugin:

1. Understand and repeat the goal of service scanning and the information you can use: you should understand that the goal of the scanning plugin is not to establish a valid connection with scanning target, i.e. the server, but to let the server send back as many service specific messages, which are messages that are only valid in the specific service, as they can. You can use these information when constructing the message: our IP, our port, server IP, server port. Also, you can use user's input to fill in the <user defined part> of request header. 

2. There are two situations: (1) The situation is that if the document of <service name> protocol says that the server will first send a request to us, we should wait to receive this request after we established a TCP(UDP) connection with the server. Then, we check whether this request has a valid format of <service name> protocol with the document. If the format is valid, we can confirm that the server is running <service name> service. Otherwise, just output the error message. You should include the original request in the plugin output in both cases. (2) The second situation is that the RFC says we need to send message to the server first after the TCP(UDP) connection. In this case, we need to send a message that acquires server's response. If the server is able to reply with valid <service name> message, we can confirm that its running <service name>. If there's no way to acquire server's message use the above information, we can also send a message that will cause server to reply with a constant error message that only occurs in <service name>. If we can receive this certain error message, we can also confirm that the server is running <service name> service. You should always include the original message from server in both cases. The <service name> protocol will suit only one of the two situations. Please figure out which situation it is.

3. According to the results of above steps, please select the message types to send and receive to get as much information from servers as you can. Then, give out the execution logic of codes for sending and receiving these messages.

4. Provide the formats of messages to send and receive. Please also provide the format of possible error responses. As mentioned in step 1, the scan is considered as successful if you receive a valid response or a legitimate error response. You should always output all raw messages from server.

5. Synthesize the above steps to provide a coding outline. Any experienced programmer should be able to implement the complete scanning tool according to your coding outline only.

Please start now.
"""

chat = Chat(api_key=api_key, url=url, system_prompt=system_prompt)

flag, result = chat.sendMessage(message=user_prompt, model=model)