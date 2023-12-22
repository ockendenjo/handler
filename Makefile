.PHONY: test

test:
	bash -c 'diff -u <(echo -n) <(go fmt $(go list ./...))'
	go vet ./...
	go test ./... -v && (echo "\nResult=OK") || (echo "\nResult=FAIL" && exit 1)
