import http.server
import io
import pathlib
import subprocess
import tempfile
import urllib.parse
import zipfile
import threading
import gzip
import tarfile

MAX_BYTES = 100 * 1024 * 1024
OFFICE = {'.docx', '.xlsx', '.pptx', '.doc', '.xls', '.ppt', '.odt', '.ods', '.odp', '.rtf'}
AUDIO = {'.mp3', '.wav', '.ogg', '.m4a', '.flac', '.aac', '.opus'}
VIDEO = {'.mp4', '.webm', '.mov', '.mkv', '.m4v'}
slots = threading.Semaphore(1)


def office(source, root):
    ext = source.suffix
    if ext in {'.docx', '.xlsx', '.pptx', '.odt', '.ods', '.odp'}:
        with zipfile.ZipFile(source) as package:
            expected = {'.docx': 'word/document.xml', '.xlsx': 'xl/workbook.xml', '.pptx': 'ppt/presentation.xml'}.get(ext, 'content.xml')
            if expected not in package.namelist() or len(package.infolist()) > 20000 or sum(e.file_size for e in package.infolist()) > 200 * 1024 * 1024:
                raise ValueError('Invalid office package')
    profile = root / 'profile'
    profile.mkdir()
    (profile / 'registrymodifications.xcu').write_text('<?xml version="1.0"?><oor:items xmlns:oor="http://openoffice.org/2001/registry"><item oor:path="/org.openoffice.Office.Common/Security/Scripting"><prop oor:name="MacroSecurityLevel" oor:op="fuse"><value>3</value></prop></item></oor:items>')
    output_filter = 'pdf:calc_pdf_Export:{"SinglePageSheets":{"type":"boolean","value":"true"}}' if ext in {'.xlsx', '.xls', '.ods'} else 'pdf'
    subprocess.run(['soffice', '-env:UserInstallation=' + profile.as_uri(), '--headless', '--norestore', '--convert-to', output_filter, '--outdir', str(root), str(source)], check=True, timeout=80, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    result = source.with_suffix('.pdf').read_bytes()
    if not result.startswith(b'%PDF-'):
        raise ValueError('Invalid PDF')
    return result, 'application/pdf'


def archive(source):
    import libarchive
    output, count, size, seen = io.BytesIO(), 0, 0, set()
    if source.suffix == '.gz':
        with gzip.open(source, 'rb') as compressed:
            raw = compressed.read(200 * 1024 * 1024 + 1)
        if len(raw) > 200 * 1024 * 1024:
            raise ValueError('Archive too large')
        try:
            with tarfile.open(fileobj=io.BytesIO(raw)):
                pass
        except tarfile.ReadError:
            try:
                raw.decode('utf-8')
                name = 'contents.txt'
            except UnicodeDecodeError:
                name = 'contents.bin'
            with zipfile.ZipFile(output, 'w', zipfile.ZIP_DEFLATED) as target:
                target.writestr(name, raw)
            return output.getvalue(), 'application/zip'
    with zipfile.ZipFile(output, 'w', zipfile.ZIP_DEFLATED) as target, libarchive.file_reader(str(source)) as entries:
        for entry in entries:
            if entry.isdir:
                continue
            name = entry.pathname
            while name.startswith('./'):
                name = name[2:]
            if not entry.isfile or not name or name.startswith('/') or any(p in ('', '.', '..') for p in name.split('/')) or any(c in name for c in ('\\', '\x00', ':')) or name.lower() in seen:
                raise ValueError('Unsafe archive path')
            seen.add(name.lower())
            count += 1
            if count > 2000:
                raise ValueError('Too many entries')
            with target.open(name, 'w') as item:
                for block in entry.get_blocks():
                    size += len(block)
                    if size > 200 * 1024 * 1024:
                        raise ValueError('Archive too large')
                    item.write(block)
    return output.getvalue(), 'application/zip'


class Handler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(200 if self.path == '/healthz' else 404)
        self.end_headers()

    def do_POST(self):
        parsed = urllib.parse.urlparse(self.path)
        query = urllib.parse.parse_qs(parsed.query)
        name = pathlib.Path(query.get('name', [''])[0]).name
        ext = pathlib.Path(name).suffix.lower()
        output_format = query.get('format', ['pdf'])[0]
        try:
            size = int(self.headers.get('Content-Length', '0'))
        except ValueError:
            size = 0
        valid = (output_format == 'pdf' and ext in OFFICE or output_format == 'png' and ext in {'.avif', '.bmp'} or output_format == 'mp3' and ext in AUDIO or output_format == 'mp4' and ext in VIDEO or output_format == 'zip' and ext in {'.tar', '.gz', '.tgz', '.7z'})
        if parsed.path != '/convert' or not valid or not 0 < size <= MAX_BYTES:
            self.send_error(400, 'Unsupported conversion request')
            return
        if not slots.acquire(timeout=5):
            self.send_error(503, 'Converter busy')
            return
        try:
            with tempfile.TemporaryDirectory(prefix='judex-preview-') as directory:
                root = pathlib.Path(directory)
                source = root / ('input' + ext)
                raw = self.rfile.read(size)
                if len(raw) != size:
                    raise ValueError('Incomplete upload')
                source.write_bytes(raw)
                if output_format == 'pdf':
                    data, mime = office(source, root)
                elif output_format == 'zip':
                    data, mime = archive(source)
                else:
                    destination = root / ('preview.' + output_format)
                    args = ['ffmpeg', '-v', 'error', '-nostdin', '-protocol_whitelist', 'file,pipe', '-threads', '1', '-i', str(source), '-threads', '1']
                    if output_format == 'png':
                        args += ['-frames:v', '1']
                        mime = 'image/png'
                    elif output_format == 'mp3':
                        args += ['-map', '0:a:0', '-vn', '-codec:a', 'libmp3lame', '-b:a', '160k']
                        mime = 'audio/mpeg'
                    else:
                        args += ['-map', '0:v:0', '-map', '0:a:0?', '-codec:v', 'libx264', '-preset', 'veryfast', '-pix_fmt', 'yuv420p', '-vf', 'scale=trunc(iw/2)*2:trunc(ih/2)*2', '-codec:a', 'aac', '-movflags', '+faststart']
                        mime = 'video/mp4'
                    subprocess.run(args + ['-fs', str(MAX_BYTES + 1), str(destination)], check=True, timeout=80, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
                    data = destination.read_bytes()
                if not data or len(data) > MAX_BYTES:
                    raise ValueError('Preview size limit')
                self.send_response(200)
                self.send_header('Content-Type', mime)
                self.send_header('Content-Length', str(len(data)))
                self.end_headers()
                self.wfile.write(data)
        except Exception:
            self.send_error(422, 'File conversion failed')
        finally:
            slots.release()

http.server.ThreadingHTTPServer(('0.0.0.0', 9081), Handler).serve_forever()
