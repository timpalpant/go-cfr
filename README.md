go-cfr
======

go-cfr is a package that implements several forms of [Counterfactual Regret Minimization](https://www.quora.com/What-is-an-intuitive-explanation-of-counterfactual-regret-minimization) in [Go](https://golang.org).
CFR can be used to solve for an approximate [Nash Equilibrium](https://en.wikipedia.org/wiki/Nash_equilibrium)
in an imperfect information [extensive-form game](https://en.wikipedia.org/wiki/Extensive-form_game).

[![GoDoc](https://godoc.org/github.com/timpalpant/go-cfr?status.svg)](http://godoc.org/github.com/timpalpant/go-cfr)
[![Build Status](https://travis-ci.org/timpalpant/go-cfr.svg?branch=master)](https://travis-ci.org/timpalpant/go-cfr)
[![Coverage Status](https://coveralls.io/repos/timpalpant/go-cfr/badge.svg?branch=master&service=github)](https://coveralls.io/github/timpalpant/go-cfr?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/timpalpant/go-cfr)](https://goreportcard.com/badge/github.com/timpalpant/go-cfr)

## Usage

To use CFR, you must implement the extensive-form game tree for your game,
by implementing the `ExtensiveFormGame` interface.

An implementation of [Kuhn Poker](https://en.wikipedia.org/wiki/Kuhn_poker) is included
as an example.

```
package main

import (
    "github.com/timpalpant/go-cfr"
    "github.com/timpalpant/go-cfr/kuhn"
)

func main() {
    poker := kuhn.NewGame()
    cfr := cfr.NewVanilla(poker)
    nIter := 10000
    for i := 1; i <= nIter; i++ {
        expectedValue += cfr.Run(poker.RootNode())
    }

    expectedValue /= nIter
    fmt.Printf("Expected value is: %v\n", expectedValue)
}
```

## Contributing

Pull requests and issues are welcomed!

Some additional CFR algorithms that could be implemented:

- Linear CFR: https://arxiv.org/abs/1809.04040
- CFR+: https://arxiv.org/abs/1407.5042
- Monte Carlo CFR:
    - Chance Sampling CFR: http://mlanctot.info/files/papers/nips09mccfr.pdf
    - External Sampling CFR: http://mlanctot.info/files/papers/nips09mccfr.pdf
    - Average Strategy CFR: https://papers.nips.cc/paper/4569-efficient-monte-carlo-counterfactual-regret-minimization-in-games-with-many-player-actions.pdf
- Deep CFR: https://arxiv.org/abs/1811.00164
- Single Deep CFR: https://arxiv.org/abs/1901.07621

## License

go-cfr is released under the [GNU Lesser General Public License, Version 3.0](https://www.gnu.org/licenses/lgpl-3.0.en.html).
