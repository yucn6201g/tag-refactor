"use strict";
// The helper of tag-refactor for TypeScript CDK apps (contract C4, version 1).
//
// It reads one request in JSON on standard input, reads the TypeScript sources
// of the app with the app's own node_modules/typescript, and writes one result
// in JSON on standard output. It only reads; it never writes a file. It
// writes no source text but the text of the keys and values of tag sites;
// what it says on standard error starts with "tag-refactor-helper:".
//
// The shapes it understands are the ones of v0.2.0 U1 (BR2.2):
//   - in the entry file of cdk.json, new <Stack>(<app>, '<id>', ...), where
//     <app> was made with new App();
//   - in the constructor of that stack class, const <v> = new <C>(this, '<id>')
//     and this.<x> = new <C>(this, '<id>'), also with a scope made that way;
//   - Tags.of(<this | v | this.x>).add(<key>, <value>) and .remove(<key>).
// Anything else that touches tags is listed in "unsupported".
//
// Positions are UTF-8 byte offsets: TypeScript counts UTF-16 code units.

const fs = require("fs");
const path = require("path");

const VERSION = 1;
const SKIPPED = new Set(["node_modules", "cdk.out"]);
const MAX_FILES = 5000;

function main() {
  let request;
  try {
    request = JSON.parse(fs.readFileSync(0, "utf8"));
  } catch (e) {
    return fail("internal", "the request could not be read");
  }
  if (!request || request.version !== VERSION) {
    return fail("internal", "the request is not of version " + VERSION);
  }
  let ts;
  try {
    ts = require(require.resolve("typescript", { paths: [request.appDir] }));
  } catch (e) {
    return fail("noTypeScript", "typescript is not found from the app");
  }
  let result;
  try {
    result = analyze(ts, request);
  } catch (e) {
    return fail(
      "internal",
      "internal error: " + (e && e.name) + " " + (e && e.message),
    );
  }
  process.stdout.write(JSON.stringify(result));
}

function fail(kind, message) {
  process.stderr.write("tag-refactor-helper: " + message + "\n");
  process.stdout.write(
    JSON.stringify({
      version: VERSION,
      tagSites: [],
      constructSites: [],
      insertSites: [],
      unsupported: [],
      error: { kind: kind },
    }),
  );
}

function analyze(ts, request) {
  const appDir = request.appDir;
  const overrides = new Map();
  for (const f of request.files || []) {
    overrides.set(f.path, f.content);
  }
  const out = {
    version: VERSION,
    typescriptVersion: ts.version,
    tagSites: [],
    constructSites: [],
    insertSites: [],
    unsupported: [],
    error: null,
  };
  const entry = entryFile(appDir);
  const files = new Map();
  for (const rel of sourceFiles(ts, appDir, entry)) {
    const text = overrides.has(rel)
      ? overrides.get(rel)
      : fs.readFileSync(path.join(appDir, rel), "utf8");
    const kind = /\.[cm]?js$/.test(rel) ? ts.ScriptKind.JS : ts.ScriptKind.TS;
    const sf = ts.createSourceFile(
      rel,
      text,
      ts.ScriptTarget.Latest,
      true,
      kind,
    );
    if (sf.parseDiagnostics && sf.parseDiagnostics.length > 0) {
      const at = sf.parseDiagnostics[0].start || 0;
      out.unsupported.push({
        file: rel,
        line: lineOf(sf, at),
        reason: "parseError",
      });
      continue;
    }
    files.set(rel, new Source(ts, rel, sf));
  }
  const classes = classIndex(ts, files);
  const entrySource = entry && files.get(entry);
  if (entrySource) {
    for (const stack of stackInstances(ts, entrySource)) {
      out.constructSites.push({
        constructPath: stack.id,
        file: entrySource.rel,
        line: stack.line,
        constructId: stack.id,
        from: "source",
      });
      const found = classes.get(stack.className) || [];
      if (found.length === 1) {
        readClass(ts, found[0], stack.id, out);
      } else if (found.length > 1) {
        out.unsupported.push({
          file: entrySource.rel,
          line: stack.line,
          reason: "unsupportedShape",
        });
      }
    }
  }
  for (const source of files.values()) {
    for (const line of source.unseenTagCalls()) {
      out.unsupported.push({
        file: source.rel,
        line: line,
        reason: "unsupportedShape",
      });
    }
  }
  markShared(out.tagSites);
  return out;
}

// Source is one parsed file with what is needed to turn positions into bytes.
class Source {
  constructor(ts, rel, sf) {
    this.ts = ts;
    this.rel = rel;
    this.sf = sf;
    this.text = sf.text;
    this.seen = new Set();
  }

  byteOffset(pos) {
    return Buffer.byteLength(this.text.slice(0, pos), "utf8");
  }

  // unseenTagCalls returns the lines of Tags.of(...) calls no stack reached.
  unseenTagCalls() {
    const ts = this.ts;
    const lines = [];
    const visit = (node) => {
      if (tagCall(ts, node) && !this.seen.has(node.pos)) {
        lines.push(lineOf(this.sf, node.getStart(this.sf)));
      }
      ts.forEachChild(node, visit);
    };
    visit(this.sf);
    return lines;
  }
}

function lineOf(sf, pos) {
  return sf.getLineAndCharacterOfPosition(pos).line + 1;
}

// entryFile returns the file the app command of cdk.json names.
function entryFile(appDir) {
  let command = "";
  try {
    command =
      JSON.parse(fs.readFileSync(path.join(appDir, "cdk.json"), "utf8")).app ||
      "";
  } catch (e) {
    return "";
  }
  for (const field of String(command).split(/\s+/)) {
    const word = field.replace(/^["']|["']$/g, "");
    if (/\.(ts|js|mjs|cjs)$/.test(word)) {
      return toSlash(path.normalize(word));
    }
  }
  return "";
}

// sourceFiles lists the files to read, relative to the app with "/": the .ts
// files of tsconfig.json (or of the folder when there is none) and the entry,
// without node_modules, cdk.out and declaration files (NFR3.9).
function sourceFiles(ts, appDir, entry) {
  let names = [];
  const config = path.join(appDir, "tsconfig.json");
  if (fs.existsSync(config)) {
    const read = ts.readConfigFile(config, ts.sys.readFile);
    if (!read.error) {
      names = ts.parseJsonConfigFileContent(
        read.config,
        ts.sys,
        appDir,
      ).fileNames;
    }
  } else {
    names = walk(appDir);
  }
  const rels = new Set();
  for (const name of names) {
    const rel = toSlash(path.relative(appDir, path.resolve(appDir, name)));
    if (keep(rel) && /\.ts$/.test(rel) && !/\.d\.ts$/.test(rel)) {
      rels.add(rel);
    }
  }
  if (entry && keep(entry) && fs.existsSync(path.join(appDir, entry))) {
    rels.add(entry);
  }
  const list = Array.from(rels).sort();
  return list.slice(0, MAX_FILES);
}

function keep(rel) {
  if (rel === "" || rel.startsWith("../") || path.isAbsolute(rel)) {
    return false;
  }
  return !rel.split("/").some((part) => SKIPPED.has(part));
}

function walk(dir) {
  const found = [];
  const visit = (d) => {
    for (const e of fs.readdirSync(d, { withFileTypes: true })) {
      if (e.isDirectory() && !SKIPPED.has(e.name) && !e.name.startsWith(".")) {
        visit(path.join(d, e.name));
      } else if (e.isFile() && e.name.endsWith(".ts")) {
        found.push(path.join(d, e.name));
      }
      if (found.length > MAX_FILES) {
        return;
      }
    }
  };
  visit(dir);
  return found;
}

function toSlash(p) {
  return p.split(path.sep).join("/");
}

// classIndex maps a class name to the declarations of that name.
function classIndex(ts, files) {
  const classes = new Map();
  for (const source of files.values()) {
    for (const st of source.sf.statements) {
      if (ts.isClassDeclaration(st) && st.name) {
        const list = classes.get(st.name.text) || [];
        list.push({ source: source, node: st });
        classes.set(st.name.text, list);
      }
    }
  }
  return classes;
}

// stackInstances finds new <Class>(<app>, '<id>', ...) in the entry file.
function stackInstances(ts, source) {
  const apps = new Set();
  const stacks = [];
  const visit = (node) => {
    if (
      ts.isVariableDeclaration(node) &&
      ts.isIdentifier(node.name) &&
      node.initializer &&
      ts.isNewExpression(node.initializer) &&
      /(^|\.)App$/.test(node.initializer.expression.getText(source.sf))
    ) {
      apps.add(node.name.text);
    }
    if (
      ts.isNewExpression(node) &&
      node.arguments &&
      node.arguments.length >= 2 &&
      ts.isIdentifier(node.arguments[0]) &&
      apps.has(node.arguments[0].text) &&
      stringLiteral(ts, node.arguments[1]) &&
      ts.isIdentifier(node.expression)
    ) {
      stacks.push({
        className: node.expression.text,
        id: node.arguments[1].text,
        line: lineOf(source.sf, node.getStart(source.sf)),
      });
    }
    ts.forEachChild(node, visit);
  };
  visit(source.sf);
  return stacks;
}

function stringLiteral(ts, node) {
  return (
    node &&
    (ts.isStringLiteral(node) || ts.isNoSubstitutionTemplateLiteral(node))
  );
}

// readClass reads the constructor of a stack class used with the path
// stackPath: the constructs it makes and the tags it adds.
function readClass(ts, cls, stackPath, out) {
  const source = cls.source;
  const sf = source.sf;
  const ctor = cls.node.members.find((m) => ts.isConstructorDeclaration(m));
  if (!ctor || !ctor.body) {
    return;
  }
  const vars = new Map();
  const members = new Map();
  const scopeOf = (node) => {
    if (!node) {
      return null;
    }
    if (node.kind === ts.SyntaxKind.ThisKeyword) {
      return stackPath;
    }
    if (ts.isIdentifier(node)) {
      return vars.has(node.text) ? vars.get(node.text) : null;
    }
    if (
      ts.isPropertyAccessExpression(node) &&
      node.expression.kind === ts.SyntaxKind.ThisKeyword
    ) {
      return members.has(node.name.text) ? members.get(node.name.text) : null;
    }
    return null;
  };
  const visit = (node) => {
    if (
      ts.isNewExpression(node) &&
      node.arguments &&
      node.arguments.length >= 2 &&
      stringLiteral(ts, node.arguments[1])
    ) {
      const scope = scopeOf(node.arguments[0]);
      if (scope !== null) {
        const id = node.arguments[1].text;
        const p = scope + "/" + id;
        bind(ts, node, p, vars, members);
        out.constructSites.push({
          constructPath: p,
          file: source.rel,
          line: lineOf(sf, node.getStart(sf)),
          constructId: id,
          from: "source",
        });
      }
    }
    const call = tagCall(ts, node);
    if (call) {
      source.seen.add(node.pos);
      const site = tagSite(ts, source, node, call, scopeOf(call.target));
      if (site) {
        out.tagSites.push(site);
      } else {
        out.unsupported.push({
          file: source.rel,
          line: lineOf(sf, node.getStart(sf)),
          reason: "unsupportedShape",
        });
      }
    }
    ts.forEachChild(node, visit);
  };
  ts.forEachChild(ctor.body, visit);
}

// bind remembers the variable or the member a new construct goes into.
function bind(ts, node, p, vars, members) {
  const parent = node.parent;
  if (
    parent &&
    ts.isVariableDeclaration(parent) &&
    parent.initializer === node &&
    ts.isIdentifier(parent.name)
  ) {
    vars.set(parent.name.text, p);
  } else if (
    parent &&
    ts.isBinaryExpression(parent) &&
    parent.right === node &&
    parent.operatorToken.kind === ts.SyntaxKind.EqualsToken &&
    ts.isPropertyAccessExpression(parent.left) &&
    parent.left.expression.kind === ts.SyntaxKind.ThisKeyword
  ) {
    members.set(parent.left.name.text, p);
  }
}

// tagCall recognizes Tags.of(<target>).add(...) and .remove(...).
function tagCall(ts, node) {
  if (
    !ts.isCallExpression(node) ||
    !ts.isPropertyAccessExpression(node.expression)
  ) {
    return null;
  }
  const op = node.expression.name.text;
  const of = node.expression.expression;
  if ((op !== "add" && op !== "remove") || !ts.isCallExpression(of)) {
    return null;
  }
  if (
    !ts.isPropertyAccessExpression(of.expression) ||
    of.expression.name.text !== "of"
  ) {
    return null;
  }
  const tags = of.expression.expression;
  const isTags =
    (ts.isIdentifier(tags) && tags.text === "Tags") ||
    (ts.isPropertyAccessExpression(tags) && tags.name.text === "Tags");
  if (!isTags || of.arguments.length !== 1) {
    return null;
  }
  return { op: op, target: of.arguments[0] };
}

// tagSite makes the site of one tag call, or null when its scope or key cannot
// be told.
function tagSite(ts, source, node, call, scope) {
  const args = node.arguments;
  if (
    scope === null ||
    args.length < 1 ||
    (call.op === "add" && args.length < 2)
  ) {
    return null;
  }
  const statement = enclosingStatement(ts, node);
  return {
    useFile: source.rel,
    useLine: lineOf(source.sf, node.getStart(source.sf)),
    op: call.op,
    scopeConstructPath: scope,
    keyEdit: editOf(ts, source, args[0]),
    valueEdit: call.op === "add" ? editOf(ts, source, args[1]) : null,
    removeEdit: {
      file: source.rel,
      start: source.byteOffset(statement.getStart(source.sf)),
      end: source.byteOffset(statement.getEnd()),
      shape: "literal",
      text: "",
      sharedWith: [],
    },
  };
}

function enclosingStatement(ts, node) {
  let n = node;
  while (n.parent && !ts.isExpressionStatement(n) && !ts.isBlock(n.parent)) {
    n = n.parent;
  }
  return n;
}

// editOf returns where a key or a value is written: the string itself, the
// string of a const of the same file, or the expression.
function editOf(ts, source, node) {
  if (stringLiteral(ts, node)) {
    return literalEdit(source, node, "literal");
  }
  if (ts.isIdentifier(node)) {
    const init = sameFileConst(ts, source.sf, node.text);
    if (init) {
      return literalEdit(source, init, "sameFileConst");
    }
  }
  return {
    file: source.rel,
    start: source.byteOffset(node.getStart(source.sf)),
    end: source.byteOffset(node.getEnd()),
    shape: "expression",
    text: "",
    sharedWith: [],
  };
}

function literalEdit(source, node, shape) {
  return {
    file: source.rel,
    start: source.byteOffset(node.getStart(source.sf)),
    end: source.byteOffset(node.getEnd()),
    shape: shape,
    text: node.text,
    sharedWith: [],
  };
}

// sameFileConst finds the string a const of the file is set to.
function sameFileConst(ts, sf, name) {
  let found = null;
  const visit = (node) => {
    if (
      !found &&
      ts.isVariableDeclarationList(node) &&
      (node.flags & ts.NodeFlags.Const) !== 0
    ) {
      for (const d of node.declarations) {
        if (
          ts.isIdentifier(d.name) &&
          d.name.text === name &&
          stringLiteral(ts, d.initializer)
        ) {
          found = d.initializer;
        }
      }
    }
    ts.forEachChild(node, visit);
  };
  visit(sf);
  return found;
}

// markShared fills sharedWith: the scopes of the other sites that write the
// same range of a file (a stack class used twice, or a const used twice).
function markShared(sites) {
  const byRange = new Map();
  const add = (edit, scope) => {
    if (!edit || edit.shape === "expression") {
      return;
    }
    const key = edit.file + "\u0000" + edit.start + "\u0000" + edit.end;
    const list = byRange.get(key) || [];
    list.push({ edit: edit, scope: scope });
    byRange.set(key, list);
  };
  for (const site of sites) {
    add(site.keyEdit, site.scopeConstructPath);
    add(site.valueEdit, site.scopeConstructPath);
  }
  for (const list of byRange.values()) {
    for (const item of list) {
      item.edit.sharedWith = list
        .filter((other) => other !== item)
        .map((other) => other.scope);
    }
  }
}

main();
