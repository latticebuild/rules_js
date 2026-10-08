import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import {fileURLToPath} from 'node:url';
const args = process.argv.slice(2);
const input = JSON.parse(fs.readFileSync(args[args.indexOf('--input') + 1], 'utf8'));
const output = args[args.indexOf('--output') + 1];
const manifests = new Map();
const visited = new Set();
const instances = new Map();
const inventory = args.includes("--inventory");
const origin = path.dirname(fileURLToPath(import.meta.url));
function locate(name, from) {
  for (let dir = from; ; dir = path.dirname(dir)) {
    const candidate = path.join(dir, 'node_modules', name, 'package.json');
    try { return [fs.realpathSync(candidate), JSON.parse(fs.readFileSync(candidate, 'utf8'))]; }
    catch (error) { if (!['ENOENT', 'ENOTDIR'].includes(error.code)) throw error; }
    if (path.dirname(dir) === dir) return null;
  }
}
function allowed(list, value) {
  return !list || (!list.includes('!' + value) && (!list.some(v => !v.startsWith('!')) || list.includes(value)));
}
function visit(name, from, optional = false) {
  const entry = locate(name, from);
  if (!entry) { if (optional) return; throw new Error('Missing declared dependency ' + name); }
  const [file, manifest] = entry;
  if (!allowed(manifest.os, process.platform) || !allowed(manifest.cpu, process.arch)) {
    if (optional) return; throw new Error('Unsupported required dependency ' + name);
  }
  if (visited.has(file)) return;
  visited.add(file);
  instances.set(file, manifest);
  manifests.set(manifest.name + '@' + manifest.version, crypto.createHash('sha256').update(JSON.stringify(manifest)).digest('hex'));
  const optionalDeps = manifest.optionalDependencies || {};
  for (const dependency of Object.keys(manifest.dependencies || {}).sort()) {
    visit(dependency, path.dirname(file), dependency in optionalDeps);
  }
  for (const dependency of Object.keys(optionalDeps).sort()) visit(dependency, path.dirname(file), true);
  if (inventory) for (const dependency of Object.keys(manifest.peerDependencies || {}).sort()) visit(dependency, path.dirname(file), manifest.peerDependenciesMeta?.[dependency]?.optional === true);
}
for (const dependency of input.dependencies) visit(dependency, origin);
const packages = [...manifests].sort(([a], [b]) => a.localeCompare(b));
const result = {nonce:input.nonce, node:process.version, packages};
if (inventory) {
  result.instances = [...instances].map(([file, manifest]) => {
    const root = path.dirname(file);
    const bindings = [];
    for (const kind of ['dependencies', 'optionalDependencies', 'peerDependencies']) {
      for (const name of Object.keys(manifest[kind] || {}).sort()) {
        const target = locate(name, root);
        if (target && allowed(target[1].os, process.platform) && allowed(target[1].cpu, process.arch)) {
          bindings.push([kind, name, target[1].name + '@' + target[1].version]);
        }
      }
    }
    const files = [];
    function walk(dir, prefix) {
      for (const entry of fs.readdirSync(dir, {withFileTypes:true}).sort((a,b) => a.name.localeCompare(b.name))) {
        if (entry.name === 'node_modules') continue;
        if (entry.name === 'BUILD.bazel' || entry.name.startsWith('.aspect_rules_js') || entry.name === '.bazelignore') continue;
        const absolute = path.join(dir, entry.name);
        const relative = prefix + entry.name;
        const info = fs.statSync(absolute);
        if (info.isDirectory()) walk(absolute, relative + '/');
        else if (info.isFile()) files.push([relative, crypto.createHash('sha256').update(fs.readFileSync(absolute)).digest('hex'), info.size]);
        else throw new Error('Non-regular npm payload: ' + relative);
      }
    }
    walk(root, '');
    return {name:manifest.name, version:manifest.version, bindings, files};
  }).sort((a,b) => JSON.stringify([a.name,a.version,a.bindings]).localeCompare(JSON.stringify([b.name,b.version,b.bindings])));
}
fs.mkdirSync(path.dirname(output), {recursive:true});
fs.writeFileSync(output, JSON.stringify(result) + '\n');
