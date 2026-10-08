process.stdout.write(JSON.stringify({ args: process.argv.slice(2) }));
if (process.argv.includes("--fail")) {
  // Set asynchronously, as a CLI reports failure after its run completes.
  await Promise.resolve();
  process.exitCode = 9;
}
