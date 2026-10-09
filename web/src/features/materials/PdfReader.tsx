import {useEffect, useRef, useState} from 'react';
import {Spinner} from '@heroui/react';
import {getDocument, GlobalWorkerOptions, version as pdfVersion, type PDFDocumentProxy, type RenderTask} from 'pdfjs-dist';
import worker from 'pdfjs-dist/build/pdf.worker.min.mjs?url';
import {ChevronLeft, ChevronRight, ZoomIn, ZoomOut} from 'lucide-react';
import {Button} from '../../components/ui/Button';
import {useWork} from '../work/store';
GlobalWorkerOptions.workerSrc = worker;

export function PdfReader({url, thumbnail = false, onError}: {url: string; thumbnail?: boolean; onError?: () => void}) {
  const {t} = useWork(), canvas = useRef<HTMLCanvasElement>(null), container = useRef<HTMLDivElement>(null);
  const callback = useRef(onError);callback.current = onError;
  const rendering = useRef<RenderTask | undefined>(undefined);
  const [document, setDocument] = useState<{url: string; pdf: PDFDocumentProxy}>();
  const [page, setPage] = useState(1), [zoom, setZoom] = useState(100), [failed, setFailed] = useState(false);
  const [size, setSize] = useState({width: 0, ratio: 1}), [drawn, setDrawn] = useState('');
  const pdf = document?.url === url ? document.pdf : undefined;
  const drawKey = [url, page, zoom, size.width, size.ratio, thumbnail].join(':');
  const busy = !failed && drawn !== drawKey;

  useEffect(() => {
    let alive = true;setFailed(false);setDocument(undefined);setPage(1);setDrawn('');
    const support = import.meta.env.BASE_URL + 'assets/pdf-support/' + pdfVersion + '/';
    const loading = getDocument({url, withCredentials: true, cMapUrl: support + 'cmaps/', cMapPacked: true,
      standardFontDataUrl: support + 'fonts/', wasmUrl: support + 'wasm/'});
    loading.promise.then(pdf => {if (alive) setDocument({url, pdf});}).catch(() => {
      if (alive) {setFailed(true);callback.current?.();}
    });
    return () => {alive = false;void loading.destroy();};
  }, [url]);

  useEffect(() => {
    const element = container.current;if (!element) return;
    const measure = () => {
      const style = getComputedStyle(element), width = Math.max(0, element.clientWidth - parseFloat(style.paddingLeft) - parseFloat(style.paddingRight));
      const ratio = window.devicePixelRatio || 1;
      setSize(previous => previous.width === width && previous.ratio === ratio ? previous : {width, ratio});
    };
    const observer = new ResizeObserver(measure);observer.observe(element);window.addEventListener('resize', measure);measure();
    return () => {observer.disconnect();window.removeEventListener('resize', measure);};
  }, []);

  useEffect(() => {
    if (!pdf || !canvas.current || !size.width) return;
    let cancelled = false, task: RenderTask | undefined;
    void (async () => {
      // Finish cancellation before sharing the canvas with a new page/size.
      const previous = rendering.current;previous?.cancel();await previous?.promise.catch(() => {});
      const source = await pdf.getPage(page);if (cancelled || !canvas.current) return;
      const target = canvas.current, base = source.getViewport({scale: 1});
      const scale = (thumbnail ? size.width / base.width : Math.min(1, size.width / base.width)) * zoom / 100;
      const viewport = source.getViewport({scale});
      target.width = Math.floor(viewport.width * size.ratio);target.height = Math.floor(viewport.height * size.ratio);
      target.style.width = viewport.width + 'px';target.style.height = viewport.height + 'px';
      task = source.render({canvas: target, canvasContext: target.getContext('2d')!, viewport,
        transform: size.ratio === 1 ? undefined : [size.ratio, 0, 0, size.ratio, 0, 0]});
      rendering.current = task;await task.promise;
      if (!cancelled) setDrawn(drawKey);
    })().catch(error => {
      if (!cancelled && error?.name !== 'RenderingCancelledException') {setFailed(true);callback.current?.();}
    });
    return () => {cancelled = true;task?.cancel();};
  }, [pdf, page, zoom, size.width, size.ratio, thumbnail, drawKey]);

  return <div className={thumbnail ? 'judex-material-pdf-thumbnail' : 'judex-material-pdf'} data-render-state={failed ? 'failed' : busy ? 'pending' : 'ready'}>
    {!thumbnail && pdf && <div className="judex-material-reader-toolbar">
      <Button size="sm" isIconOnly aria-label={t('matPrevious')} disabled={page === 1 || busy} onPress={() => setPage(p => p - 1)}><ChevronLeft/></Button><span>{page} / {pdf.numPages}</span>
      <Button size="sm" isIconOnly aria-label={t('matNext')} disabled={page === pdf.numPages || busy} onPress={() => setPage(p => p + 1)}><ChevronRight/></Button>
      <Button size="sm" isIconOnly aria-label={t('matZoomOut')} disabled={zoom <= 50 || busy} onPress={() => setZoom(z => z - 25)}><ZoomOut/></Button><span>{zoom}%</span>
      <Button size="sm" isIconOnly aria-label={t('matZoomIn')} disabled={zoom >= 200 || busy} onPress={() => setZoom(z => z + 25)}><ZoomIn/></Button>
    </div>}
    <div className="judex-material-pdf-canvas" ref={container} aria-busy={busy}>
      <canvas ref={canvas} className={busy ? 'judex-material-canvas-pending' : ''}/>
      {busy && <div className="judex-material-reading-state" role="status"><Spinner size="sm" aria-hidden="true"/><span>{t('matReading')}</span></div>}
      {failed && <p role="alert">{t('matReadError')}</p>}
    </div>
  </div>;
}
