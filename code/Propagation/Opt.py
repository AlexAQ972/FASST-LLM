from Chat import Chat
from pathlib import Path

DEFAULT_SYSTEM_PROMPT = '''
You are the optimizer module in a differentiable LLM-based network scanning system.

Goal
Your task is to update the current value of a variable (in text format), based on the provided gradient information, in order to reduce the previously observed loss.

Input
You will receive:
(1) The current value of a variable (text format) in <Label> ... </Label>
(2) A gradient description enclosed in <Grad> ... </Grad>. This gradient may include multiple parts corresponding to sub-components of the variable, such as:
    - For code: "packet-sending logic", "response-parsing logic"
    - For a Scanning Tree: "packet construction", "sending", "parsing"

Each part will have a directional suggestion for improvement, or it may be marked as "Zero", meaning no change is needed.

Output
Your job is to return the **updated version of the variable**, based on all non-zero gradients. If all gradients are Zero, return the variable unchanged.

Output format
Return the updated variable value in its label.

Instructions
(1) Carefully read and identify each part of the gradient (<Grad>).
(2) For each subcomponent with a non-zero suggestion, apply a suitable modification to the corresponding part of the input.
(3) Maintain the structure and logic of the original input as much as possible.
(4) Only change what is indicated by the gradients.
(5) If the gradient is Zero for all parts, do not modify the input variable.
'''

PROMPT_OPT = '''
You are given a prompt and a set of gradient-based suggestions that reflect issues or improvements identified during use. Your task is to revise and optimize the prompt to reduce the associated loss, while following the strict constraints below.

Input:
(1) The original prompt, which includes formatting specifications.
(2) A list of suggestions (gradient) in natural language, which point out what improvements should be made.

Instructions:
- Incorporate the suggestions **into the prompt** to improve clarity, completeness, or effectiveness.
- You may rephrase existing sentences to integrate suggestions naturally, but:
  - Do **not modify any formatting requirements** (e.g., tag structures such as `<A> ... </A>`) already in the prompt.
  - Do **not remove** existing functional constraints unless the gradient explicitly says so.
- Your output should be a fully updated version of the prompt, not just a diff or patch.
- The changes should feel natural and cohesive, as if the prompt was written that way from the beginning.
- If the gradient contains "Zero", return the original prompt unchanged.

Output Format:
<Prompt>
[Your updated prompt here]
</Prompt>

Input:
<Prompt>
{Prompt}
</Prompt>

<Grad>
{Grad}
</Grad>
'''

class Optimizer(Chat):
    def __init__(self, api_key, url, model, system_prompt=None, title='title_A'):
        if system_prompt is None:
            system_prompt = DEFAULT_SYSTEM_PROMPT
        super().__init__(api_key, url, system_prompt, title)
        self.model = model
        self.api_key = api_key

    def step_prompt(self, prompt, grad):
        user_prompt = PROMPT_OPT
        user_prompt = user_prompt.replace('{Prompt}', prompt)
        user_prompt = user_prompt.replace('{Grad}', grad)
        
        flag, result = self.sendMessage(user_prompt, self.model)
        if flag is False:
            return False, result
        
        start = result.find('<Prompt>')
        end = result.find('</Prompt>')
        
        if start == -1 or end == -1:
            return False, result
        
        return True, result[start:end]

