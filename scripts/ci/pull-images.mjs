import {spawnSync} from 'node:child_process';
import {setTimeout} from 'node:timers/promises';
import {pathToFileURL} from 'node:url';

const attempts = 3;
const transientResponse = /HTTP (?:status[: ]+)?(?:408|429|500|502|503|504)\b|\b(?:Internal Server Error|Bad Gateway|Service Unavailable|Gateway Time-?out|Too Many Requests|toomanyrequests)\b|\b(?:i\/o timeout|TLS handshake timeout|connection reset|connection refused|unexpected EOF|context deadline exceeded|temporary failure in name resolution)\b/i;
const transientCodes = new Set(['ETIMEDOUT', 'ECONNRESET', 'ECONNREFUSED', 'EAI_AGAIN']);

function dockerPull(image) {
  const result = spawnSync('docker', ['pull', image], {
    encoding: 'utf8', windowsHide: true, timeout: 120_000, maxBuffer: 8 << 20,
  });
  if (result.stdout) process.stdout.write(result.stdout);
  if (result.stderr) process.stderr.write(result.stderr);
  return result;
}

export async function pullImages(images, {
  pull = dockerPull,
  wait = setTimeout,
  onRetry = ({image, attempt, delayMs}) => console.warn(`Temporary pull failure for ${image}; retry ${attempt + 1}/${attempts} in ${delayMs}ms`),
} = {}) {
  if (!images.length) throw new Error('Provide at least one Docker image');
  for (const image of images) {
    for (let attempt = 1; attempt <= attempts; attempt++) {
      const result = pull(image);
      if (result.status === 0) break;
      const diagnostic = [result.stderr, result.stdout, result.error?.message].filter(Boolean).join('\n');
      const transient = transientCodes.has(result.error?.code) || transientResponse.test(diagnostic);
      if (!transient || attempt === attempts) {
        throw new Error(`Docker pull failed for ${image} (${attempt}/${attempts}): ${diagnostic || result.signal || result.status}`, {cause: result.error});
      }
      const delayMs = 2000 * 2 ** (attempt - 1);
      onRetry({image, attempt, delayMs});
      await wait(delayMs);
    }
  }
}

if (process.argv[1] && pathToFileURL(process.argv[1]).href === import.meta.url) {
  pullImages(process.argv.slice(2)).catch(error => {
    console.error(error.message);
    process.exitCode = 1;
  });
}
