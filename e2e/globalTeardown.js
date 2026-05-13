const fs = require('fs');
const path = require('path');

const PID_FILE = path.join(__dirname, '.server.pid');

module.exports = async function globalTeardown() {
  try {
    const pid = fs.readFileSync(PID_FILE, 'utf8').trim();
    process.kill(parseInt(pid), 'SIGKILL');
    fs.unlinkSync(PID_FILE);
  } catch {}
};
