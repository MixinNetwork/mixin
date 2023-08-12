# should add `option go_package = "types";` to *.proto

protoc --proto_path=. --go_out=. --go_opt=paths=source_relative *.proto
