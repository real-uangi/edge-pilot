import { readFile, readdir } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import ts from "typescript";

const root = fileURLToPath(new URL("../src", import.meta.url));
async function walk(directory) {
  const entries = await readdir(directory, { withFileTypes: true });
  const files = await Promise.all(
    entries.map((entry) => {
      const path = resolve(directory, entry.name);
      return entry.isDirectory() ? walk(path) : [path];
    }),
  );
  return files.flat();
}
const paths = (await walk(root)).filter((path) => /\.(css|tsx?)$/.test(path));
const sources = new Map(
  await Promise.all(
    paths.map(async (path) => [path, await readFile(path, "utf8")]),
  ),
);
const failures = [];
let references = 0;
for (const [path, source] of sources) {
  if (!/\.tsx?$/.test(path)) continue;
  const file = ts.createSourceFile(path, source, ts.ScriptTarget.Latest, true);
  const modules = new Map();
  for (const statement of file.statements) {
    if (
      !ts.isImportDeclaration(statement) ||
      !ts.isStringLiteral(statement.moduleSpecifier)
    )
      continue;
    const name = statement.importClause?.name?.text;
    const target = statement.moduleSpecifier.text;
    if (!name || !target.endsWith(".module.css")) continue;
    const cssPath = resolve(dirname(path), target);
    const css = sources.get(cssPath);
    if (css === undefined) {
      failures.push(`${path}: missing CSS module ${target}`);
      continue;
    }
    const selectors = css.replace(/\/\*[\s\S]*?\*\//g, "");
    modules.set(
      name,
      new Set(
        [...selectors.matchAll(/\.([a-zA-Z_][\w-]*)/g)].map(
          (match) => match[1],
        ),
      ),
    );
  }
  function visit(node) {
    let owner;
    let name;
    if (
      ts.isPropertyAccessExpression(node) &&
      ts.isIdentifier(node.expression)
    ) {
      owner = node.expression.text;
      name = node.name.text;
    } else if (
      ts.isElementAccessExpression(node) &&
      ts.isIdentifier(node.expression) &&
      ts.isStringLiteral(node.argumentExpression)
    ) {
      owner = node.expression.text;
      name = node.argumentExpression.text;
    }
    if (owner && modules.has(owner)) {
      references += 1;
      if (!modules.get(owner).has(name)) {
        const { line } = file.getLineAndCharacterOfPosition(
          node.getStart(file),
        );
        failures.push(
          `${path}:${line + 1}: undefined CSS class ${owner}.${name}`,
        );
      }
    }
    ts.forEachChild(node, visit);
  }
  visit(file);
}
const css = [...sources]
  .filter(([path]) => path.endsWith(".css"))
  .map(([, source]) => source)
  .join("\n");
const definitions = new Set(
  [...css.matchAll(/(--ep-[\w-]+)\s*:/g)].map((match) => match[1]),
);
for (const [, source] of sources) {
  for (const match of source.matchAll(/var\((--ep-[\w-]+)/g)) {
    if (!definitions.has(match[1]))
      failures.push(`Undefined design token ${match[1]}`);
  }
}
const global = sources.get(resolve(root, "styles/global.css"));
const colors = new Map(
  [...global.matchAll(/(--ep-[\w-]+):\s*(#[\da-fA-F]{6})/g)].map((match) => [
    match[1],
    match[2],
  ]),
);
function luminance(hex) {
  const values = [1, 3, 5]
    .map((offset) => Number.parseInt(hex.slice(offset, offset + 2), 16) / 255)
    .map((value) =>
      value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4,
    );
  return values[0] * 0.2126 + values[1] * 0.7152 + values[2] * 0.0722;
}
const pairs = ["--ep-text", "--ep-text-muted", "--ep-text-dim"].flatMap(
  (text) =>
    ["--ep-bg", "--ep-surface", "--ep-surface-lowest", "--ep-surface-mid"].map(
      (background) => [text, background],
    ),
);
pairs.push(
  ["--ep-primary-ink", "--ep-primary"],
  ["--ep-secondary", "--ep-secondary-soft"],
  ["--ep-warning", "--ep-warning-soft"],
  ["--ep-danger", "--ep-danger-soft"],
);
for (const [text, background] of pairs) {
  const [low, high] = [
    luminance(colors.get(text)),
    luminance(colors.get(background)),
  ].sort((a, b) => a - b);
  const ratio = (high + 0.05) / (low + 0.05);
  if (ratio < 4.5)
    failures.push(
      `${text} on ${background}: contrast ${ratio.toFixed(2)} is below 4.5:1`,
    );
}
if (failures.length) {
  console.error([...new Set(failures)].join("\n"));
  process.exitCode = 1;
} else {
  console.log(
    `Checked ${references} literal CSS references, ${definitions.size} design tokens and ${pairs.length} text contrast pairs.`,
  );
}
