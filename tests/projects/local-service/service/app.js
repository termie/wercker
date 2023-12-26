const http = require('http')
http.createServer((req, res) => {
  res.end('hello from another service')
}).listen(8000)
