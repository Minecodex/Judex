// APIError → i18n key 的统一映射。VALIDATION_ERROR 优先用 details.fields
// 还原字段级原因（后端 apierrors.Fields），避免只给"请检查输入"的笼统提示。
import type { Key } from "../../i18n";
import { APIError } from "../../lib/api/client";

type ValidationDetails = { fields?: { path?: string; code?: string }[] };

function validationKey(error: APIError): Key | null {
  const fields = (error.details as ValidationDetails | undefined)?.fields;
  const first = fields?.[0];
  if (!first) return null;
  if (first.path === "password" || first.path === "newPassword") return "errPasswordLength";
  if (first.path === "displayName") return "errDisplayNameLength";
  if (first.path === "email") return "errEmailFormat";
  return null;
}

export function errorKey(error: unknown): Key | null {
  if (!(error instanceof APIError)) return "errNetwork";
  switch (error.code) {
    case "INVALID_CREDENTIALS":
    case "UNAUTHENTICATED":
      return "errInvalidCredentials";
    case "SESSION_EXPIRED":
      return "errSessionExpired";
    case "RATE_LIMITED":
      return "errRateLimited";
    case "EMAIL_IN_USE":
      return "errEmailInUse";
    case "VALIDATION_ERROR":
      return validationKey(error) ?? "errValidation";
    case "FORBIDDEN":
      return "errForbidden";
    case "NOT_IMPLEMENTED":
      return "errServer";
    default:
      return error.status >= 500 ? "errServer" : "errNetwork";
  }
}
