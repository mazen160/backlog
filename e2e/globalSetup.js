const { execFileSync, spawn } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

const BIN = path.resolve(__dirname, '../backlog');
const PORT = 8181;
const PID_FILE = path.join(__dirname, '.server.pid');
const DB_FILE = path.join(__dirname, '.test.db');

module.exports = async function globalSetup() {
  // Kill any leftover server
  try {
    const pid = fs.readFileSync(PID_FILE, 'utf8').trim();
    process.kill(parseInt(pid), 'SIGKILL');
  } catch {}

  // Fresh DB
  if (fs.existsSync(DB_FILE)) fs.unlinkSync(DB_FILE);

  const env = { ...process.env, BACKLOG_DB: DB_FILE };

  // Seed data
  const run = (args) => execFileSync(BIN, ['--db', DB_FILE, ...args], { env, stdio: 'pipe' });

  run(['project', 'add', 'Alpha', '--alias', 'alpha']);
  run(['project', 'add', 'Beta',  '--alias', 'beta']);
  run(['task', 'add', '--project', 'alpha', '--title', 'Fix login bug',   '--type', 'bug',     '--priority', '2', '--status', 'doing']);
  run(['task', 'add', '--project', 'alpha', '--title', 'Add dark mode',   '--type', 'feature', '--priority', '3']);
  run(['task', 'add', '--project', 'alpha', '--title', 'Write unit tests','--type', 'chore',   '--priority', '3', '--description', 'Cover auth module']);
  run(['memory', 'add', 'Decided to use SQLite', '--project', 'alpha', '--tag', 'decision,arch']);
  run(['doc', 'add', '--project', 'alpha', '--title', 'Architecture Overview']);

  // Start server
  const srv = spawn(BIN, ['--db', DB_FILE, 'web', '--no-browser', '--port', String(PORT)], {
    env,
    detached: true,
    stdio: 'ignore',
  });
  srv.unref();
  fs.writeFileSync(PID_FILE, String(srv.pid));

  // Wait for server to be ready
  await new Promise((resolve, reject) => {
    const start = Date.now();
    const tryFetch = () => {
      fetch(`http://localhost:${PORT}/api/projects`)
        .then(() => resolve())
        .catch(() => {
          if (Date.now() - start > 8000) return reject(new Error('Server did not start'));
          setTimeout(tryFetch, 200);
        });
    };
    tryFetch();
  });
};
