import assert from "node:assert/strict";
import {webcrypto} from "node:crypto";
import {readFileSync} from "node:fs";
import {createRequire} from "node:module";
import test from "node:test";
import {runInNewContext} from "node:vm";
import ts from "typescript";

const require = createRequire(import.meta.url);
const state = "callback+state/with=padding&value";
const dependencies = {
  "@/auth/Util": {getStateFromQueryParams: () => state},
  "@/lib/setting": {isCustomOAuthType: () => false, getOriginUrl: () => "https://login.example.com"},
};
const module = {exports: {}};
const source = readFileSync(new URL("../src/auth/Provider.ts", import.meta.url), "utf8");
const {outputText} = ts.transpileModule(source, {
  compilerOptions: {module: ts.ModuleKind.CommonJS, esModuleInterop: true},
});
// Keep the actual URL builder and PKCE code; replace only browser state and UI dependencies.
runInNewContext(outputText, {
  module, exports: module.exports,
  require: (name) => dependencies[name] ?? require(name),
  crypto: webcrypto, Uint8Array, btoa, URL, URLSearchParams,
  localStorage: {setItem() {}},
});

test("Bilibili authorization uses the current endpoint and encoded query parameters", () => {
  for (const forcedRedirectOrigin of [undefined, "https://login.example.com/path?tenant=a&next=b"]) {
    const result = new URL(module.exports.getAuthUrl(
      {name: "application", forcedRedirectOrigin},
      {type: "Bilibili", name: "provider", clientId: "client+id&value"},
      "signin",
    ));
    assert.equal(result.origin + result.pathname, "https://account.bilibili.com/pc/account-pc/auth/oauth");
    assert.equal(result.hash, "");
    assert.equal(result.searchParams.get("client_id"), "client+id&value");
    assert.equal(result.searchParams.get("gourl"), `${forcedRedirectOrigin ?? "https://login.example.com"}/callback`);
    assert.equal(result.searchParams.get("state"), state);
    assert.equal(result.searchParams.has("return_url"), false);
  }
});
