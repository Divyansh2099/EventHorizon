# Ensure docker is running
docker run -it --rm `
    -v "${PWD}/tests/autobahn/fuzzingclient.json:/config/fuzzingclient.json" `
    -v "${PWD}/tests/autobahn/reports:/config/reports" `
    --network="host" `
    crossbario/autobahn-testsuite `
    wstest -m fuzzingclient -s /config/fuzzingclient.json
