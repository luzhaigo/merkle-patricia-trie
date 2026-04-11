package spawner

import (
	"fmt"
	"math/rand/v2"
	"net"
)

var (
	DefaultMinPort = 4000
	DefaultMaxPort = 4999
)

func FindFreePort(min, max int) (port int, outErr error) {
	if min == 0 && max == 0 {
		min = DefaultMinPort
		max = DefaultMaxPort
	}

	if min > max {
		return 0, fmt.Errorf("min port is greater than max port")
	}

	port = rand.IntN(max-min+1) + min

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err == nil {
		ln.Close()
		return port, nil
	} else {
		outErr = err
	}

	for p := min; p < max+1; p++ {
		if p == port {
			continue
		}
		ln, err = net.Listen("tcp", fmt.Sprintf(":%d", p))
		if err == nil {
			ln.Close()
			return p, nil
		} else {
			outErr = err
		}
	}

	return 0, fmt.Errorf("no free port in range %d-%d: %w", min, max, outErr)
}
