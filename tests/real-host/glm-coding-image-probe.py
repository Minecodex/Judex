"""Owned synthetic screenshot only; no secrets or image bytes are logged."""
import base64,json,pathlib,subprocess,urllib.request,urllib.error
env=json.loads(pathlib.Path('.cache/real-host/20261012-closure/environment.json').read_text())
def kube(*args): return json.loads(subprocess.check_output(['kubectl',*args],creationflags=subprocess.CREATE_NO_WINDOW))
ns=kube('get','namespace',env['namespace'],'-o','json')['metadata']
assert ns['uid']==env['uid'] and ns['labels']['judex.dev/test-run']==env['runId']
secret=kube('-n',env['namespace'],'get','secret','real-model-key','-o','json')
assert secret['metadata']['labels']['judex.dev/test-run']==env['runId']
key=base64.b64decode(secret['data']['JUDEX_GLM_CODING_KEY']).decode()
folder=pathlib.Path(env['localRoot'])/'A-MiniIssue/review/qa13-theme-output'
file=next(iter(sorted(folder.glob('*.png'))))
body={'model':'glm-5.3-flash','stream':False,'max_tokens':128,'thinking':{'type':'disabled'},'messages':[{'role':'user','content':[{'type':'text','text':'这是本轮自有MiniIssue测试页面截图。用一句话描述页面主题，不输出其他内容。'},{'type':'image_url','image_url':{'url':'data:image/png;base64,'+base64.b64encode(file.read_bytes()).decode()}}]}]}
request=urllib.request.Request('https://open.bigmodel.cn/api/coding/paas/v4/chat/completions',data=json.dumps(body).encode(),headers={'Content-Type':'application/json','Authorization':'Bearer '+key})
try:
 with urllib.request.urlopen(request,timeout=45) as response: result=json.load(response)
 choice=result['choices'][0]
 print(json.dumps({'scope':'Coding Plan image admission, separate from platform calls','model':'glm-5.3-flash','http':200,'finishReason':choice.get('finish_reason'),'description':choice['message'].get('content','')[:250]},ensure_ascii=False))
except urllib.error.HTTPError as error:
 payload=json.load(error);print(json.dumps({'http':error.code,'providerCode':payload.get('error',{}).get('code'),'message':payload.get('error',{}).get('message','').replace(key,'[redacted]')[:250]},ensure_ascii=False))
except Exception as error: print(json.dumps({'probeErrorType':type(error).__name__}))
