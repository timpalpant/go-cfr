package cfr

import (
	"encoding/gob"
	"io"

	"github.com/golang/glog"
	"github.com/pkg/errors"
)

type cfrHeader struct {
	Params     Params
	NumPlayers int
}

type playerHeader struct {
	Player      int
	NumPolicies int
}

type strategy struct {
	InfoSet     string
	RegretSum   []float32
	StrategySum []float32
}

// Load CFR state from the given reader.
func Load(r io.Reader) (*CFR, error) {
	dec := gob.NewDecoder(r)
	hdr := cfrHeader{}
	if err := dec.Decode(&hdr); err != nil {
		return nil, errors.Wrap(err, "error decoding CFR header")
	}

	glog.Infof("Loading policies for %d players", hdr.NumPlayers)
	strategyProfile := make(map[int]map[string]*policy)
	for player := 0; player < hdr.NumPlayers; player++ {
		playerHdr := playerHeader{}
		if err := dec.Decode(&playerHdr); err != nil {
			return nil, errors.Wrap(err, "error decoding player header")
		}

		glog.Infof("Loading %d policies for player %d", playerHdr.NumPolicies, player)
		policyMap := make(map[string]*policy)
		for i := 0; i < playerHdr.NumPolicies; i++ {
			s := strategy{}
			if err := dec.Decode(&s); err != nil {
				return nil, errors.Wrap(err, "error decoding policy")
			}

			p := &policy{
				regretSum:   s.RegretSum,
				strategy:    make([]float32, len(s.RegretSum)),
				strategySum: s.StrategySum,
			}

			p.nextStrategy()
			policyMap[s.InfoSet] = p
		}

		glog.Infof("Loaded %d policies for player %d", len(policyMap), player)
		strategyProfile[playerHdr.Player] = policyMap
	}

	return &CFR{
		params:          hdr.Params,
		strategyProfile: strategyProfile,
		slicePool:       &floatSlicePool{},
	}, nil
}

// Marshal CFR state to the given writer.
func (c *CFR) Save(w io.Writer) error {
	enc := gob.NewEncoder(w)

	hdr := cfrHeader{
		Params:     c.params,
		NumPlayers: len(c.strategyProfile),
	}

	if err := enc.Encode(hdr); err != nil {
		return errors.Wrap(err, "error encoding CFR header")
	}

	for player, policies := range c.strategyProfile {
		playerHdr := playerHeader{Player: player, NumPolicies: len(policies)}
		if err := enc.Encode(playerHdr); err != nil {
			return errors.Wrap(err, "error encoding player header")
		}

		for infoSet, policy := range policies {
			s := strategy{
				InfoSet:     infoSet,
				RegretSum:   policy.regretSum,
				StrategySum: policy.strategySum,
			}

			if err := enc.Encode(s); err != nil {
				return errors.Wrap(err, "error encoding policy")
			}
		}
	}

	return nil
}
