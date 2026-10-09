import {APIError} from '../../lib/api/client';
import {errorKey} from '../auth/errors';
import type {Key} from '../../i18n';
export function materialErrorKey(error:unknown):Key {
  return error instanceof APIError&&error.code==='VERSION_CONFLICT'?'matVersionChanged':errorKey(error)??'matReadError';
}
