module github.com/dedis/student20_rabyt

go 1.15

require (
	go.dedis.ch/dela v0.0.0-20201221101342-e1cc707548c7
	go.dedis.ch/simnet v0.3.10
	golang.org/x/tools v0.0.0-20200904185747-39188db58858
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1
)

replace (
	go.dedis.ch/simnet => /home/cache-nez/epfl/dedis-semester-project/simnet
)