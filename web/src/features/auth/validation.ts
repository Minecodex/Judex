// 注册/找回密码的本地字段校验，规则与后端 internal/identity/service.go
// ValidateRegistration / recovery.go 保持一致：displayName 1–80（去首尾空白、
// 按字符计）、email 非空且含 @ 且 ≤254、password 12–128。返回 i18n key。
import type { Key } from "../../i18n";

export function validateDisplayName(value: string): Key | null {
  const length = [...value.trim()].length;
  if (length < 1 || length > 80) return "errDisplayNameLength";
  return null;
}

export function validateEmail(value: string): Key | null {
  if (value.length === 0 || value.length > 254 || !value.includes("@")) {
    return "errEmailFormat";
  }
  return null;
}

export function validatePassword(value: string): Key | null {
  if (value.length < 12 || value.length > 128) return "errPasswordLength";
  return null;
}
