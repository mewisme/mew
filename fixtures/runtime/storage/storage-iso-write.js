// Write localStorage data for isolation test.
var fs = require('fs');
localStorage.setItem('iso', 'value-a');
fs.writeFileSync('output.txt', localStorage.getItem('iso'));
