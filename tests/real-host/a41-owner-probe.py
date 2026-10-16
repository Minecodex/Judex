"""Operator-assisted U0 read-only probe; each result comes from actual data."""
import hashlib
import json
import pathlib
import re
import socket
import sqlite3
import subprocess
import sys
import urllib.error
import urllib.request
import zipfile
from datetime import datetime, timezone

repo = pathlib.Path(__file__).resolve().parents[2]
environment = json.loads((repo / '.cache/real-host/20261012-closure/environment.json').read_text(encoding='utf-8'))
local = pathlib.Path(environment['localRoot']) / 'A-MiniIssue'
review = local / 'review/A41-owner-original'
fault = local / 'fault'
receipt = json.loads((local / 'receipts/KA06-A41-delivery.json').read_text(encoding='utf-8'))
state = json.loads((fault / 'scenario.json').read_text(encoding='utf-8'))
if state['namespaceUID'] != environment['uid'] or state['runId'] != environment['runId'] or pathlib.Path(state['root']).resolve() != fault.resolve():
    raise RuntimeError('Owned fault target required')
checks = []
sha = lambda data: hashlib.sha256(data).hexdigest()

def check(name, valid, detail):
    checks.append({'name': name, 'passed': bool(valid), 'detail': detail})

originals = []
for material in receipt['materials']:
    original = review / pathlib.Path(material['file']).name
    originals.append({'versionId': material['versionId'], 'actualSha256': sha(original.read_bytes()), 'expectedSha256': material['sha256']})
check('C1 immutable original SHA', all(i['actualSha256'] == i['expectedSha256'] for i in originals), originals)
archive = review / 'a41-diagnosis-evidence.zip'
expected = {'scenario.json', 'config.json', 'config-before.json', 'before-http.json', 'before.log', 'injected-startup.log', 'controller-status.json', 'probes.json'}
members = []
with zipfile.ZipFile(archive) as package:
    for entry in package.infolist():
        relative = pathlib.PurePosixPath(entry.filename)
        if relative.is_absolute() or '..' in relative.parts or ':' in entry.filename or '\\' in entry.filename:
            raise RuntimeError('Unsafe evidence ZIP path')
        if not entry.is_dir():
            extracted = review / 'evidence' / entry.filename
            members.append({'name': entry.filename, 'matchesExtracted': extracted.is_file() and sha(package.read(entry)) == sha(extracted.read_bytes())})
check('C2 ZIP members and extracted bytes', len(members) == 8 and {pathlib.PurePosixPath(m['name']).name for m in members} == expected and all(m['matchesExtracted'] for m in members), members)
evidence = review / 'evidence/a41-evidence'
before_config = json.loads((evidence / 'config-before.json').read_text(encoding='utf-8'))
config = json.loads((fault / 'config.json').read_text(encoding='utf-8'))
check('C3 still injected configuration', pathlib.Path(config['db']).is_dir() and pathlib.Path(before_config['db']).resolve() == pathlib.Path(state['db']).resolve() and config == json.loads((evidence / 'config.json').read_text(encoding='utf-8')), {'currentDbIsDirectory': pathlib.Path(config['db']).is_dir(), 'beforeDbIsFile': pathlib.Path(before_config['db']).is_file()})
db = pathlib.Path(state['db'])
before_hash = sha(db.read_bytes())
check('C4 database original bytes', before_hash == state['beforeDbSha256'], {'size': db.stat().st_size, 'sha256': before_hash, 'expectedSha256': state['beforeDbSha256']})
failure = None
try:
    connection = sqlite3.connect(config['db'])
    connection.close()
except sqlite3.OperationalError as error:
    failure = str(error)
check('C5 SQLite directory failure', failure == 'unable to open database file', {'actualError': failure})
connection = sqlite3.connect(db.as_uri() + '?mode=ro', uri=True)
connection.row_factory = sqlite3.Row
try:
    rows = [dict(row) for row in connection.execute('SELECT id,title,description,status,severity,created_at,updated_at FROM issues ORDER BY id')]
finally:
    connection.close()
before_http = json.loads((evidence / 'before-http.json').read_text(encoding='utf-8'))
check('C6 all preserved row fields', len(rows) == 4 and rows == before_http['issues']['body']['items'], {'actualRows': rows, 'expectedCount': before_http['issues']['body']['total']})
tcp_refused = False
tcp_error = None
try:
    with socket.create_connection(('127.0.0.1', 18482), timeout=5):
        pass
except ConnectionRefusedError:
    tcp_refused = True
except OSError as error:
    tcp_error = type(error).__name__
http_refused = False
opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
try:
    with opener.open('http://127.0.0.1:18482/api/health', timeout=5):
        pass
except urllib.error.URLError as error:
    http_refused = isinstance(error.reason, ConnectionRefusedError)
check('C7 actual unavailable HTTP/TCP', tcp_refused and http_refused and state['activeProcess'] is None, {'tcpRefused': tcp_refused, 'httpRefused': http_refused, 'otherTcpError': tcp_error, 'recordedProcess': state['activeProcess'], 'note': 'prior2s Python socket timed out; Node controller independently returned ECONNREFUSED, this5s probe records its own actual result'})
source = pathlib.Path(state['source']).resolve()
source_results = []
for relative, expected_hash in state['sourceHashes'].items():
    target = (source / relative).resolve()
    if not target.is_relative_to(source):
        raise RuntimeError('Unexpected source path')
    source_results.append({'file': relative, 'matches': target.is_file() and sha(target.read_bytes()) == expected_hash})
check('C8 actual frozen source hashes', len(source_results) == 30 and all(item['matches'] for item in source_results) and (source / 'VERSION').read_text(encoding='utf-8').strip() == 'v0.1.1', source_results)
log = (evidence / 'injected-startup.log').read_text(encoding='utf-8')
frames = []
for match in re.finditer(r'^\s*File "([^"\n]+)", line (\d+)[^\n]*\n\s*([^\n]+)', log, re.MULTILINE):
    frame_path = pathlib.Path(match.group(1)).resolve()
    if frame_path.is_relative_to(source):
        line = int(match.group(2))
        actual = frame_path.read_text(encoding='utf-8').splitlines()[line - 1].strip()
        frames.append({'file': frame_path.name, 'line': line, 'matchesFrozenStatement': actual == match.group(3).strip()})
check('C9 actual traceback/frozen statements', {(f['file'], f['line']) for f in frames} == {('server.py', 43), ('server.py', 61), ('db.py', 74), ('db.py', 82), ('db.py', 92)} and all(f['matchesFrozenStatement'] for f in frames) and 'OperationalError: unable to open database file' in log, frames)
timeline_paths = [fault / 'history/config-before.json', fault / 'history/before.log', db, fault / 'history/before-http.json', fault / 'config.json', fault / 'history/injected-startup.log', fault / 'scenario.json']
times = [item.stat().st_mtime_ns for item in timeline_paths]
check('C10 actual ordered timeline', times == sorted(times) and sha(db.read_bytes()) == before_hash, [{'file': str(item.relative_to(fault)), 'mtimeNs': value} for item, value in zip(timeline_paths, times)])
result = {'observedAt': datetime.now(timezone.utc).isoformat(), 'actor': 'U0 operator-assisted independent verification', 'scriptSha256': sha(pathlib.Path(__file__).read_bytes()), 'instanceId': state['instanceId'], 'checks': checks, 'passed': sum(c['passed'] for c in checks), 'total': len(checks), 'allPassed': all(c['passed'] for c in checks), 'scope': 'read-only; no fixture config/source/database changes, no process start/stop; failed model probe preserved separately'}
output = review / 'owner-probes-operator.json'
output.write_text(json.dumps(result, ensure_ascii=False, indent=2) + '\n', encoding='utf-8')
print(json.dumps({'output': str(output), 'passed': result['passed'], 'total': result['total'], 'allPassed': result['allPassed'], 'failedChecks': [c['name'] for c in checks if not c['passed']]}, ensure_ascii=False))
sys.exit(0 if result['allPassed'] else 1)
