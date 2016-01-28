package main

import (
	"bufio"
	"io"
	"log"
	"os"
	"time"

	"github.com/brentp/bix"
	"github.com/brentp/cgotbx"
	"github.com/brentp/vcfgo"
	"github.com/brentp/xopen"
)

func check(e error) {
	if e != nil {
		log.Fatal(e)
	}
}

type location struct {
	chrom string
	start int
	end   int
}

func (s location) RefName() string {
	return s.chrom
}
func (s location) Start() int {
	return s.start
}
func (s location) End() int {
	return s.end
}

func main() {

	f, err := xopen.Ropen(os.Args[1])
	check(err)
	vcf, err := vcfgo.NewReader(f, true)
	check(err)
	tbx, err := bix.New(os.Args[1])
	check(err)
	var rdr io.Reader
	tot := 0
	t0 := time.Now()
	for {
		v := vcf.Read()
		if v == nil {
			break
		}

		rdr, err = tbx.ChunkedReader(location{v.Chrom(), int(v.Start()), int(v.Start()) + 1})
		check(err)
		brdr := bufio.NewReader(rdr)
		for _, err := brdr.ReadString('\n'); err == nil; _, err = brdr.ReadString('\n') {
			tot += 1
		}
	}
	log.Println(tot, time.Since(t0).Seconds())

	main2()
}

func main2() {

	/*
		f, err := os.Create("q.pprof")
		if err != nil {
			panic(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	*/

	t, err := cgotbx.New(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	var rdr io.Reader
	for i := 0; i < 1000; i++ {
		tot := 0
		f, err := xopen.Ropen(os.Args[1])
		check(err)
		vcf, err := vcfgo.NewReader(f, true)
		check(err)
		t0 := 0
		for {
			v := vcf.Read()
			if v == nil {
				break
			}

			ts := time.Now()
			for k := 0; k < 100; k++ {
				rdr, err = t.Get(v.Chrom(), int(v.Start()), int(v.Start())+1)
				check(err)
				brdr := bufio.NewReader(rdr)
				//fmt.Fprintln(os.Stderr, v.Chrom(), v.Start(), v.Start()+1)
				j := 0
				for l, err := brdr.ReadString('\n'); err == nil; l, err = brdr.ReadString('\n') {
					//fmt.Fprintln(os.Stderr, "...", l[:20])
					_ = l
					tot += 1
					j += 1
				}
				if j == 0 {
					log.Fatal("should have found something")
				}
			}
			t0 += int(time.Since(ts).Nanoseconds())
		}
		log.Println(tot, float64(t0)*1e-9)
	}
}
