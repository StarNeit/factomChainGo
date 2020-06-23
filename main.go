package main

import (
	"fmt"
	"github.com/Factom-Asset-Tokens/factom"
	"github.com/pegnet/pegnet/modules/grader"
	"github.com/sirupsen/logrus"
)

var (
	OPRChain                              = factom.NewBytes32("cffce0f409ebba4ed236d49d89c70e4bd1f1367d86402a3363366683265a242d")
	GradingV2Activation            uint32 = 210330
	PEGFreeFloatingPriceActivation uint32 = 222270
	V4OPRUpdate                    uint32 = 231620
)

func main() {
	cl := factom.NewClient()
	cl.FactomdServer = "https://api.factomd.net/v2"

	dblock := new(factom.DBlock)
	dblock.Height = 237975

	if err := dblock.Get(nil, cl); err != nil {
		fmt.Println("error: ", err)
		return
	}

	oprEBlock := dblock.EBlock(OPRChain)
	if oprEBlock != nil {
		if err := multiFetch(oprEBlock, cl); err != nil {
			fmt.Println("error: ", err)
			return
		}
	}

	gradedBlock, err := Grade(oprEBlock)

	if err != nil {
		fmt.Println("error: ", err)
		return
	} else if gradedBlock != nil {
		winners := gradedBlock.Winners()
		if 0 < len(winners) {
			fmt.Println("winner rate:", winners[0].OPR.GetOrderedAssetsUint())
		} else {
			fmt.Println("block not graded, no winners")
		}
	}
}

func multiFetch(eblock *factom.EBlock, c *factom.Client) error {
	err := eblock.Get(nil, c)
	if err != nil {
		return err
	}

	work := make(chan int, len(eblock.Entries))
	defer close(work)
	errs := make(chan error)
	defer close(errs)

	for i := 0; i < 8; i++ {
		go func() {
			// TODO: Fix the channels such that a write on a closed channel never happens.
			//		For now, just kill the worker go routine
			defer func() {
				recover()
			}()

			for j := range work {
				errs <- eblock.Entries[j].Get(nil, c)
			}
		}()
	}

	for i := range eblock.Entries {
		work <- i
	}

	count := 0
	for e := range errs {
		count++
		if e != nil {
			// If we return, we close the errs channel, and the working go routine will
			// still try to write to it.
			return e
		}
		if count == len(eblock.Entries) {
			break
		}
	}

	return nil
}

func Grade(block *factom.EBlock) (grader.GradedBlock, error) {
	if block == nil {
		// TODO: Handle the case where there is no opr block.
		// 		Must delay conversions if this happens
		return nil, nil
	}

	if *block.ChainID != OPRChain {
		return nil, fmt.Errorf("trying to grade a non-opr chain")
	}

	ver := uint8(1)
	if block.Height >= GradingV2Activation {
		ver = 2
	}
	if block.Height >= PEGFreeFloatingPriceActivation {
		ver = 3
	}
	if block.Height >= V4OPRUpdate {
		ver = 4
	}

	var prevWinners []string = []string{
		"3dd854aeb2d49f85",
		"86e971804fde3e6f",
		"101713d912fa651d",
		"fbb11bcaffd56d20",
		"dd3b35fb3927fb00",
		"47d2c8fde11934eb",
		"400e113adf147342",
		"22aedf7364080bd7",
		"c0f278a121482c22",
		"d41559e08eb0cbe1",
		"77686695b5c1bdbf",
		"20309006f2001e13",
		"a10a51a606d32f36",
		"69f4723d26c8f71c",
		"f6a2f3e56ae56ccd",
		"9fee4dbc582125a6",
		"c51017ad4fe1f0ca",
		"f9a2d8ac26f9e406",
		"d4bc08670fb00662",
		"8b4f33c7942976aa",
		"1186d3a3585b730c",
		"ffc797560715352f",
		"402e7cd2b4c068ef",
		"6ede211447bd442a",
		"1ca4e140457e0b7d"}

	g, err := grader.NewGrader(ver, int32(block.Height), prevWinners)
	if err != nil {
		return nil, err
	}

	for _, entry := range block.Entries {
		extids := make([][]byte, len(entry.ExtIDs))
		for i := range entry.ExtIDs {
			extids[i] = entry.ExtIDs[i]
		}
		// ignore bad opr errors
		err = g.AddOPR(entry.Hash[:], extids, entry.Content)
		if err != nil {
			// This is a noisy debug print
			logrus.WithError(err).WithFields(logrus.Fields{"hash": entry.Hash.String()}).Debug("failed to add opr")
		}
	}

	return g.Grade(), nil
}
