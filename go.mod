module github.com/mises-id/mainnet

go 1.16

require (
	github.com/cosmos/cosmos-sdk v0.44.6
	github.com/mises-id/mises-tm v0.0.0-20211229034748-9cc59047a831
	github.com/tendermint/go-amino v0.16.0
	github.com/tendermint/tendermint v0.34.16
)

replace google.golang.org/grpc => google.golang.org/grpc v1.42.0

replace github.com/gogo/protobuf => github.com/regen-network/protobuf v1.3.3-alpha.regen.1

replace github.com/tendermint/tm-db => github.com/mises-id/tm-db v0.6.5-0.20210822095222-e1ff1e0dc734

replace github.com/cosmos/iavl => github.com/mises-id/iavl v0.17.4-0.20211207035003-f9d26e6150db

replace github.com/tendermint/tendermint => github.com/mises-id/tendermint v0.34.15-0.20211207033151-1f29b59c0edf

replace github.com/cosmos/cosmos-sdk => github.com/mises-id/cosmos-sdk v0.44.6-0.20211209094558-a7c9c77cfc17
