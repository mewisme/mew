// Read localStorage data for isolation test.
var fs = require('fs');
fs.writeFileSync('output.txt', String(localStorage.getItem('iso')));
