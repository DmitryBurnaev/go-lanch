echo "==== Building /app/bin/go-lunch ===="
go build -o /app/bin/go-lunch main.go
ls -lah /app/bin/
chmod a+x /app/bin/go-lunch
/app/bin/./go-lunch
echo "==== DONE ==== "
