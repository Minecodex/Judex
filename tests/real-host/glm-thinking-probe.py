"""Tiny real Coding Plan capability probe; key stays in process memory."""
import base64, json, pathlib, subprocess, urllib.request, urllib.error
env = json.loads(pathlib.Path('.cache/real-host/20261012-closure/environment.json').read_text())
def kube(*args):
    return json.loads(subprocess.check_output(['kubectl', *args], creationflags=subprocess.CREATE_NO_WINDOW))
namespace = kube('get', 'namespace', env['namespace'], '-o', 'json')['metadata']
assert namespace['uid'] == env['uid'] and namespace['labels']['judex.dev/test-run'] == env['runId']
secret = kube('-n', env['namespace'], 'get', 'secret', 'real-model-key', '-o', 'json')
assert secret['metadata']['labels']['judex.dev/test-run'] == env['runId']
key = base64.b64decode(secret['data']['JUDEX_GLM_CODING_KEY']).decode()
for model in ['glm-5.3', 'glm-5.3-flash']:
    body = {'model': model, 'stream': False, 'max_tokens': 1024,
            'thinking': {'type': 'disabled'},
            'messages': [{'role': 'user', 'content': '仅调用 ping 工具，text=ok，不输出解释。'}],
            'tools': [{'type': 'function', 'function': {'name': 'ping', 'description': '返回测试标记', 'parameters': {'type': 'object', 'properties': {'text': {'type': 'string'}}, 'required': ['text']}}}]}
    request = urllib.request.Request('https://open.bigmodel.cn/api/coding/paas/v4/chat/completions',
             data=json.dumps(body).encode(), headers={'Content-Type': 'application/json', 'Authorization': 'Bearer '+key})
    try:
        with urllib.request.urlopen(request, timeout=45) as response:
            result = json.load(response)
        choice = result['choices'][0]
        message = choice['message']
        calls = message.get('tool_calls') or []
        print(json.dumps({'scope': 'tiny provider admission probe, separate from platform accounting', 'model': model, 'http': 200,
              'thinkingTypeRequested': 'disabled', 'finishReason': choice.get('finish_reason'),
              'toolNames': [c['function']['name'] for c in calls], 'privateReasoningPresent': bool(message.get('reasoning_content')),
              'inputTokens': result.get('usage', {}).get('prompt_tokens'), 'outputTokens': result.get('usage', {}).get('completion_tokens')}, ensure_ascii=False), flush=True)
    except urllib.error.HTTPError as error:
        payload=json.load(error)
        print(json.dumps({'model': model, 'http': error.code, 'providerCode': payload.get('error', {}).get('code')}), flush=True)
    except Exception as error:
        print(json.dumps({'model':model, 'probeErrorType': type(error).__name__}), flush=True)
