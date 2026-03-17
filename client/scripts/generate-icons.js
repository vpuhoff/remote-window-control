import sharp from "sharp";
import { readFileSync, writeFileSync } from "fs";
import { dirname, join } from "path";
import { fileURLToPath } from "url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const publicDir = join(__dirname, "..", "public");
const svgPath = join(publicDir, "icon.svg");

const svg = readFileSync(svgPath);

for (const size of [192, 512]) {
  const png = await sharp(svg)
    .resize(size, size)
    .png()
    .toBuffer();
  writeFileSync(join(publicDir, `icon-${size}.png`), png);
  console.log(`Generated icon-${size}.png`);
}
