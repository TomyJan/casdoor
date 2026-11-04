import assert from "node:assert/strict";
import {readFileSync} from "node:fs";
import {createRequire} from "node:module";
import test from "node:test";
import {runInNewContext} from "node:vm";
import {createElement} from "react";
import {renderToStaticMarkup} from "react-dom/server";
import ts from "typescript";

const require = createRequire(import.meta.url);

function loadModule(path, dependencies = {}, globals = {}) {
  const source = readFileSync(new URL(path, import.meta.url), "utf8");
  const {outputText} = ts.transpileModule(source, {
    compilerOptions: {module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX, esModuleInterop: true},
  });
  const module = {exports: {}};
  runInNewContext(outputText, {
    module, exports: module.exports,
    require: (name) => dependencies[name] ?? require(name),
    URLSearchParams, ...globals,
  });
  return module.exports;
}

function loadPromotion(globals) {
  const config = loadModule("../src/Conf.ts", {}, globals);
  // Keep the real promotion components and config; supply an admin session and
  // leave unrelated UI dependencies unloaded on the hidden rendering path.
  const promo = loadModule("../src/components/common/CloudPromo.tsx", {
    "@/Conf": config,
    "@/hooks/use-account": {useAccount: () => ({account: {isAdmin: true}})},
    "@/lib/setting": {isLocalAdminUser: (account) => account?.isAdmin === true},
    "@/lib/utils": {},
    "@/components/ui/button": {},
    "@/components/ui/dropdown-menu": {},
    "@/components/ui/sidebar": {},
  });
  return {config, promo};
}

function assertPromotionHidden(promo) {
  assert.equal(promo.showCloudPromo({isAdmin: true}), false);
  assert.equal(promo.showCloudLoginLink(), false);
  for (const [name, props] of [
    ["CloudHeaderButton", {}],
    ["CloudSidebarCard", {collapsed: false}],
    ["CloudSidebarCard", {collapsed: true}],
    ["CloudDashboardBanner", {}],
    ["CloudMenuItem", {}],
    ["CloudLoginLink", {}],
  ]) {
    assert.equal(renderToStaticMarkup(createElement(promo[name], props)), "", `${name} must render nothing`);
  }
}

test("cloud promotion stays hidden before configuration arrives, even for admins and demo pages", () => {
  const {config, promo} = loadPromotion();
  config.setConfig({showGithubCorner: true});
  assertPromotionHidden(promo);
});

test("older backend configuration without promotion settings keeps all placements hidden", () => {
  const {config, promo} = loadPromotion();
  config.setConfig({showGithubCorner: true, defaultLanguage: "zh"});
  assertPromotionHidden(promo);
});

test("invalid embedded configuration keeps all promotion placements hidden", () => {
  const {config, promo} = loadPromotion({
    document: {cookie: "", querySelector: () => ({content: "invalid JSON"})},
  });
  config.setConfig({showGithubCorner: true});
  config.initConfigFromCookie();
  assertPromotionHidden(promo);
});

test("an explicit admin preference can enable promotion and hide it again", () => {
  const {config, promo} = loadPromotion();
  config.setConfig({showGithubCorner: true, showCloudPromotion: true});
  assert.equal(promo.showCloudPromo({isAdmin: true}), true);
  assert.equal(promo.showCloudLoginLink(), true);
  config.setConfig({showCloudPromotion: false});
  assertPromotionHidden(promo);
});
