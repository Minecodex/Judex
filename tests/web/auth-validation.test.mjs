import test from "node:test";
import assert from "node:assert/strict";
import {
  validateDisplayName,
  validateEmail,
  validatePassword,
} from "../../web/src/features/auth/validation.ts";

test("displayName: 1–80 runes after trim", () => {
  assert.equal(validateDisplayName(""), "errDisplayNameLength");
  assert.equal(validateDisplayName("   "), "errDisplayNameLength");
  assert.equal(validateDisplayName("甲"), null);
  assert.equal(validateDisplayName("  kakj  "), null);
  assert.equal(validateDisplayName("甲".repeat(80)), null);
  assert.equal(validateDisplayName("甲".repeat(81)), "errDisplayNameLength");
});

test("email: non-empty, has @, max 254", () => {
  assert.equal(validateEmail(""), "errEmailFormat");
  assert.equal(validateEmail("kakjzzw.gmail.com"), "errEmailFormat");
  assert.equal(validateEmail("kakjzzw@gmail.com"), null);
  assert.equal(validateEmail("a@" + "x".repeat(252)), null);
  assert.equal(validateEmail("a@" + "x".repeat(253)), "errEmailFormat");
});

test("password: 12–128 characters", () => {
  assert.equal(validatePassword("123456"), "errPasswordLength");
  assert.equal(validatePassword("x".repeat(11)), "errPasswordLength");
  assert.equal(validatePassword("password-123"), null);
  assert.equal(validatePassword("x".repeat(12)), null);
  assert.equal(validatePassword("x".repeat(128)), null);
  assert.equal(validatePassword("x".repeat(129)), "errPasswordLength");
});
