package cgotbx_test

import (
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
	rdr, err := t.Get("1", 50000, 90000)
	c.Assert(err, IsNil)

	str, err := ioutil.ReadAll(rdr)
	c.Assert(err, IsNil)

	log.Println(string(str))

}
