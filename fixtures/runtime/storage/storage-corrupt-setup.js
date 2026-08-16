// Write initial localStorage data for corrupt recovery test.
var fs = require('fs');
localStorage.setItem('recover-test', 'hello');
fs.writeFileSync('output.txt', 'ok');
