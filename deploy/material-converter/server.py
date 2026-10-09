import http.server
import pathlib
import subprocess
import tempfile
import urllib.parse
import zipfile

class Handler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(200 if self.path == '/healthz' else 404)
        self.end_headers()

    def do_POST(self):
        parsed = urllib.parse.urlparse(self.path)
        name = pathlib.Path(urllib.parse.parse_qs(parsed.query).get('name', [''])[0]).name
        ext = pathlib.Path(name).suffix.lower()
        size = int(self.headers.get('Content-Length', '0'))
        if parsed.path != '/convert' or ext not in ('.docx', '.xlsx', '.pptx') or not 0 < size <= 100 * 1024 * 1024:
            self.send_error(400, 'Unsupported conversion request')
            return
        try:
            with tempfile.TemporaryDirectory(prefix='judex-preview-') as directory:
                root = pathlib.Path(directory)
                source = root / ('input' + ext)
                source.write_bytes(self.rfile.read(size))
                with zipfile.ZipFile(source) as package:
                    expected = {'.docx': 'word/document.xml', '.xlsx': 'xl/workbook.xml', '.pptx': 'ppt/presentation.xml'}[ext]
                    if '[Content_Types].xml' not in package.namelist() or expected not in package.namelist():
                        raise ValueError('File is not the declared Office format')
                    if sum(entry.file_size for entry in package.infolist()) > 200 * 1024 * 1024:
                        raise ValueError('Office package exceeds the conversion limit')
                profile = root / 'profile'
                profile.mkdir()
                # Disable document macros. Each conversion owns its profile and
                # temporary files; no application credentials enter this process.
                (profile / 'registrymodifications.xcu').write_text('<?xml version="1.0"?><oor:items xmlns:oor="http://openoffice.org/2001/registry"><item oor:path="/org.openoffice.Office.Common/Security/Scripting"><prop oor:name="MacroSecurityLevel" oor:op="fuse"><value>3</value></prop></item></oor:items>')
                output_filter = 'pdf'
                if ext == '.xlsx':
                    output_filter = 'pdf:calc_pdf_Export:{"SinglePageSheets":{"type":"boolean","value":"true"}}'
                subprocess.run(['soffice', '-env:UserInstallation=' + profile.as_uri(), '--headless', '--norestore', '--convert-to', output_filter, '--outdir', directory, str(source)], check=True, timeout=80, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
                data = source.with_suffix('.pdf').read_bytes()
                if not data.startswith(b'%PDF-') or len(data) > 100 * 1024 * 1024:
                    raise ValueError('Invalid PDF result')
                self.send_response(200)
                self.send_header('Content-Type', 'application/pdf')
                self.send_header('Content-Length', str(len(data)))
                self.end_headers()
                self.wfile.write(data)
        except (OSError, ValueError, zipfile.BadZipFile, subprocess.SubprocessError):
            self.send_error(422, 'Document conversion failed')

http.server.HTTPServer(('0.0.0.0', 9081), Handler).serve_forever()
