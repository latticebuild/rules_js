import { readFileSync, writeFileSync } from "node:fs";
import { answer } from "@example/answer";
const original = readFileSync("input.txt", "utf8").trim();
writeFileSync("input.txt", "changed only in scratch\n");
writeFileSync(process.argv[2], `${original}|${answer}|${process.env.MODE}\n`);
