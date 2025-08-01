from Chat import Chat

DEFAULT_SYSTEM_PROMPT = '''
You are the gradient computation module within a network security system powered by large language models (LLMs). Your task is to help optimize a specific variable in a service scanning pipeline by interpreting upstream feedback (loss description).

You will receive:
(1) The current value of a variable used during the scanning process (in textual form).
(2) A loss description that indicates whether and how this variable contributed to the failure to correctly identify the target service.

Based on this information, you should compute the "gradient" — a directional modification suggestion for this variable — that helps reduce the loss in future scans.
If the loss is not caused by this variable, the gradient of this variable should be zero.

- If the variable contributed to the loss, return a gradient suggestion using the format:
<Grad> Your directional adjustment recommendation here </Grad>

- Analyze whether the current variable value is likely responsible for the observed loss.
- If so, suggest how it can be changed (what direction, value, or structure) to improve scan accuracy.
- If no change is needed or it is not the cause, explicitly indicate that with "<Grad> Zero </Grad>".
'''

CODE_PROMPT_TEMPLATE = '''
Given the following information, compute the gradient of the loss with respect to three parts of the scan code: (1) the code that sends the packet, (2) the code that parses the response, and (3) other code.

Important Instruction:
The gradient for a part should be computed **only if that specific part is a root cause of the loss or is directly responsible for upstream gradient signals.**
Do **not** assign a gradient merely because the part is related or present in the process.  
If a part did not cause the loss or propagate any gradient, its gradient must be explicitly marked as **"Zero"**.

Scan Code:
<scan_code>
{code}
</scan_code>

Loss Description:
<loss>
{loss}
</loss>

Please return the gradient for both parts within a single <Grad> block. You need to explain how to improve each part. Clearly specify:
- The gradient for the **send packet code**
- The gradient for the **parse response code**
- The gradient for the **other code**

If either part has zero gradient (i.e., should not be changed), explicitly return "Zero" for that part.

Use the following format:
<Grad>
Send Packet Code Gradient:
[Your suggestion or "Zero"]

Parse Response Code Gradient:
[Your suggestion or "Zero"]

Other Code Gradient:
[Your suggestion or "Zero"]
</Grad>
'''

ST_PROMPT_TEMPLATE = '''
Based on the following gradients from the scan code and the Scanning Tree, compute the gradient of the loss with respect to the three components of the Scanning Tree:
(1) Packet Construction
(2) Packet Sending
(3) Response Parsing

Important Instruction:
The gradient for a part should be computed **only if that specific part is a root cause of the loss or is directly responsible for upstream gradient signals.**
Do **not** assign a gradient merely because the part is related or present in the process.  
If a part did not cause the loss or propagate any gradient, its gradient must be explicitly marked as **"Zero"**.

Scan Code Gradients:
<code_grad>
{grad}
</code_grad>

Original Scanning Tree:
<ST>
{ST}
</ST>

Please return the gradient for each of the three parts of the Scanning Tree within a single <Grad> block. You need to explain how to improve each part. Clearly specify:
- The gradient for Packet Construction
- The gradient for Packet Sending
- The gradient for Response Parsing

If any part should remain unchanged, explicitly return "Zero" for that part.

Use the following format:
<Grad>
Packet Construction Gradient:
[Your suggestion or "Zero"]

Packet Sending Gradient:
[Your suggestion or "Zero"]

Response Parsing Gradient:
[Your suggestion or "Zero"]
</Grad>
'''

PROMPT_PROMPT_TEMPLATE = '''
You are given the original prompt and a propagated gradient signal from downstream loss.

Your task is to analyze the original prompt in combination with the gradient signal and generate **gradient suggestions for prompt improvement**. These suggestions should reflect how the prompt could be augmented to better reduce loss in future executions.

Important:
- You are NOT being asked to modify the prompt directly.
- Instead, produce guidance in the form of **additive requirements or constraints** that could be appended to the prompt to improve it.
- These additions may involve clarification of format, expected logic, user/system roles, scope restrictions, or any other missing part identified by the gradient.
- If the gradient clearly indicates that no change is needed, return only the word "Zero".

Input:
(1) The original prompt.
<Prompt>
{Prompt}
</Prompt>

(2) The downstream gradient signal (describing where the prompt failed or what could be improved).
<Grad>
{Grad}
</Grad>

Your Output Format:
<Grad>
[List of suggestions or requirements that should be added to the prompt to reduce future loss]
</Grad>

'''

class Grad(Chat):
    def __init__(self, api_key, url, model, system_prompt=None, title='title_A'):
        if system_prompt is None:
            system_prompt = DEFAULT_SYSTEM_PROMPT
        self.model = model
        super().__init__(api_key, url, system_prompt, title)

    def get_code_grad(self, code, loss):
        user_prompt = CODE_PROMPT_TEMPLATE.replace('{code}', code).replace('{loss}', loss)
        flag, result = self.sendMessage(message=user_prompt, model=self.model)
        if flag is False:
            return False, result
        
        start = result.find('<Grad>') + len('<Grad>')
        end = result.find('</Grad>')

        if end == -1:
            return False, result

        return True, result[start:end]
    
    def get_ST_grad(self, ST, grad):
        user_prompt = ST_PROMPT_TEMPLATE.replace('{ST}', ST).replace('{grad}', grad)
        flag, result = self.sendMessage(message=user_prompt, model=self.model)
        if flag is False:
            return False, result
        
        start = result.find('<Grad>') + len('<Grad>')
        end = result.find('</Grad>')

        if end == -1:
            return False, result

        return True, result[start:end]
    
    def get_prompt_grad(self, prompt, grad):
        user_prompt = PROMPT_PROMPT_TEMPLATE.replace('{Grad}', grad)
        user_prompt = user_prompt.replace('{Prompt}', prompt)
        flag, result = self.sendMessage(message=user_prompt, model=self.model)
        if flag is False:
            return False, result
        
        start = result.find('<Grad>') + len('<Grad>')
        end = result.find('</Grad>')

        if end == -1:
            return False, result

        return True, result[start:end]

    

