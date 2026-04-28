import { cp, mkdir, rm } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const vendorRoot = path.join(root, "frontend/nginx/docs/assets/vendor");
const nodeModules = path.join(root, "node_modules");

const copies = [
  {
    from: "bootstrap/dist/css/bootstrap.min.css",
    to: "bootstrap/css/bootstrap.min.css",
  },
  {
    from: "bootstrap/dist/js/bootstrap.bundle.min.js",
    to: "bootstrap/js/bootstrap.bundle.min.js",
  },
  {
    from: "bootstrap-icons/font/bootstrap-icons.css",
    to: "bootstrap-icons/bootstrap-icons.css",
  },
  {
    from: "bootstrap-icons/font/fonts",
    to: "bootstrap-icons/fonts",
  },
  {
    from: "alpinejs/dist/cdn.min.js",
    to: "alpinejs/cdn.min.js",
  },
  {
    from: "htmx.org/dist/htmx.min.js",
    to: "htmx/htmx.min.js",
  },
];

async function copyVendorFile({ from, to }) {
  const source = path.join(nodeModules, from);
  const target = path.join(vendorRoot, to);

  await mkdir(path.dirname(target), { recursive: true });
  await cp(source, target, { recursive: true });
  console.log(`copied ${from} -> ${path.relative(root, target)}`);
}

await rm(vendorRoot, { recursive: true, force: true });

for (const copy of copies) {
  await copyVendorFile(copy);
}
