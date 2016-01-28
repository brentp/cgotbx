package cgotbx_test

import (
	"bufio"
	"io"
	"io/ioutil"
	"log"
	"testing"

	"github.com/brentp/cgotbx"

	. "gopkg.in/check.v1"
)

func Test(t *testing.T) { TestingT(t) }

type TBXSuite struct{}

var _ = Suite(&TBXSuite{})

func (s *TBXSuite) TestRead(c *C) {
	t, err := cgotbx.New("data/vt.norm.vcf.gz")
	c.Assert(err, IsNil)
	rdr, err := t.Get("chr1", 50000, 90000)
	c.Assert(err, IsNil)

	str, err := ioutil.ReadAll(rdr)
	c.Assert(err, IsNil)
	c.Assert(len(str) != 0, Equals, true)

}

func (s *TBXSuite) TestLongLine(c *C) {
	t, err := cgotbx.New("data/vt.norm.vcf.gz")
	c.Assert(err, IsNil)
	rdr, err := t.Get("chr1", 915415, 915428)
	c.Assert(err, IsNil)

	b := bufio.NewReader(rdr)
	line, e := b.ReadString('\n')
	log.Println(line)
	c.Assert(e, IsNil)
	line, e = b.ReadString('\n')
	log.Println(line[:20])
	c.Assert(e, IsNil)
	line, e = b.ReadString('\n')
	c.Assert(e, Equals, io.EOF)

}
