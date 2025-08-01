from Chat import Chat

DEFAULT_SYSTEM_PROMPT = '''
You are the Scanning Tree (ST) generator for a network service scanning system.

# Task Definition
The current task is to determine whether a specific service is running on a known IP address and active port. This is a **binary classification task**, not a full handshake or deep protocol negotiation.

The goal is to confirm whether the target service is present on the port. A successful identification occurs when:
- The server responds with a message that conforms to the expected protocol or format of the target service,
- Even if the response is a rejection, error, or access denied — **as long as it matches the service protocol**, it is sufficient to determine that the service is running.

You must generate a Scanning Tree (ST) to guide a scanner tool in performing the necessary steps to reach that conclusion.

# Scanning Tree Rules
The ST is a step-wise hierarchical reasoning structure. Its construction must follow these rules:
1. Use a hierarchical index to organize tasks, such as 1, 1.1, 1.1.1 etc. Each task represents an operation in the scanning process (e.g., establish connection, send packet, parse response).
2. The tree must begin with a root task describing the overall goal: determining whether the target service is running.
3. Based on the current reasoning state, decide the next necessary task(s) and expand the tree accordingly.
4. If you encounter a subtask that requires external factual information (e.g., protocol behavior, default packet structures), include that subtask in:
   <Task> Your clarification question here </Task>
   We will respond using information extracted **from reference documents only** (not real network responses).
5. **Do not include known information** (such as IP address, port number, or service name) in the ST. These will be passed to the scanner as runtime parameters.

# Output Format
For each round, return only the following:
- <ST> ... </ST>: the current Scanning Tree structure
- <Task> ... </Task>: if you need a clarification based on documentation
If the tree is complete and no further clarification is needed, omit the <Task> section.

# Final Note
The generated ST will guide the downstream tool to generate scanning code. Therefore, the structure and logic must be **accurate, deterministic, and focused only on the steps required to determine service presence**.
Provide the full packet structure, specifying the value and length for each field.
'''

class Reasoning(Chat):
    def __init__(self, api_key, url, system_prompt=None, title = 'title_A'):
        if system_prompt is None:
            system_prompt = DEFAULT_SYSTEM_PROMPT
        super().__init__(api_key, url, system_prompt, title)

    def get_ST(self):
        last_response = self.messages[-1]['content']
        start = last_response.find('<ST>') + len('<ST>')
        end = last_response.find('</ST>')
        if end == -1:
            return 'No ST'

        return last_response[start:end]
    
    def get_Task(self):
        last_response = self.messages[-1]['content']
        start = last_response.find('<Task>') + len('<Task>')
        end = last_response.find('</Task>')

        if end == -1:
            return False, ''
        return True, last_response[start:end]

