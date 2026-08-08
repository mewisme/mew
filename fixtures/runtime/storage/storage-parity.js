// Verify CJS/ESM storage globals are installed and functional.
var fs = require('fs');
var ok = typeof localStorage === 'object' &&
  typeof localStorage.getItem === 'function' &&
  typeof localStorage.setItem === 'function' &&
  typeof sessionStorage === 'object' &&
  typeof sessionStorage.getItem === 'function';
fs.writeFileSync('output.txt', ok ? 'PARITY_OK' : 'PARITY_FAIL');
