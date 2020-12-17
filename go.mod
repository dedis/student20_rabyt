module github.com/dedis/student20_rabyt

go 1.15

require (
	github.com/stretchr/testify v1.6.1
	go.dedis.ch/dela v0.0.0-20201014124135-54b9c0717601
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1
	golang.org/x/tools v0.0.0-20200904185747-39188db58858
	github.com/rs/zerolog v1.20.0
	github.com/stretchr/testify v1.6.1
	github.com/urfave/cli/v2 v2.2.0
	go.dedis.ch/dela v0.0.0-20201217112440-cfc46e5624bc
	go.dedis.ch/simnet v0.3.10
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1
)

// required until https://github.com/dedis/dela/issues/170 is fixed
replace go.dedis.ch/dela => /home/cache-nez/epfl/dedis-semester-project/dela
