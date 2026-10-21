# Run inside the owned converter image; all files are generated test material.
import io, json, pathlib, subprocess, tarfile, gzip, zipfile, shutil, os
root=pathlib.Path('/fixtures');root.mkdir(exist_ok=True)
def run(args): subprocess.run(args,check=True,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,timeout=80)
for ext,src in [('doc','docx'),('odt','docx'),('rtf','docx'),('xls','xlsx'),('ods','xlsx'),('ppt','pptx'),('odp','pptx')]:
 run(['soffice','-env:UserInstallation=file:///tmp/fixture-profile','--headless','--convert-to',ext,'--outdir',str(root),'/original/acceptance.'+src])
shutil.copy('/original/acceptance.png',root/'picture.png')
for ext in ['bmp','avif']:
 run(['ffmpeg','-y','-v','error','-i',str(root/'picture.png'),'-frames:v','1',str(root/('picture.'+ext))])
for ext in ['mp3','wav','ogg','m4a','flac','aac','opus']:
 run(['ffmpeg','-y','-v','error','-f','lavfi','-i','sine=frequency=440:duration=1','-threads','1',str(root/('audio.'+ext))])
for ext in ['mp4','webm','mov','mkv','m4v']:
 run(['ffmpeg','-y','-v','error','-f','lavfi','-i','color=c=green:s=160x120:d=1','-threads','1','-pix_fmt','yuv420p',str(root/('video.'+ext))])
(root/'budget.csv').write_text('name,amount,notes\n设计,1200,"品牌,视觉"\n研发,3400,"line one\nline two"\n',encoding='utf-8')
(root/'budget.tsv').write_text('name\tamount\n设计\t1200\n研发\t3400\n',encoding='utf-8')
(root/'gbk.csv').write_bytes('姓名,预算\n设计,1200\n'.encode('gb18030'))
(root/'flow.mmd').write_text('flowchart LR\n A[需求] --> B[设计]\n B --> C[开发]\n',encoding='utf-8')
(root/'picture.svg').write_text('<svg xmlns="http://www.w3.org/2000/svg" width="160" height="120"><rect width="160" height="120" fill="#72774f"/></svg>')
(root/'unsafe.svg').write_text('<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>')
(root/'code.ts').write_text('const budget = 1200;\n// exact evidence\nexport const team = "Judex";\n')
(root/'config.json').write_text(json.dumps({'team':'Judex','budget':1200,'review':{'required':True}}))
(root/'config.yaml').write_text('team: Judex\nbudget: 1200\n')
(root/'config.toml').write_text('team = "Judex"\nbudget = 1200\n')
(root/'config.xml').write_text('<team budget="1200">Judex</team>')
(root/'notes.markdown').write_text('# 阅读验收\n\n- 固定版本\n- 原文可回读\n')
with zipfile.ZipFile(root/'reader.zip','w') as z:
 z.write(root/'budget.csv','data/budget.csv');z.write(root/'code.ts','source/code.ts');z.write(root/'picture.png','images/picture.png')
with tarfile.open(root/'reader.tar','w') as t:t.add(root/'budget.csv',arcname='data/budget.csv')
(root/'reader.tgz').write_bytes(gzip.compress((root/'reader.tar').read_bytes()))
(root/'reader.gz').write_bytes(gzip.compress((root/'code.ts').read_bytes()))
import libarchive
os.chdir(root)
with libarchive.file_writer(str(root/'reader.7z'),'7zip') as a:a.add_files('budget.csv')
