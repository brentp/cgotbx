package cgotbx

//http://dominik.honnef.co/posts/2015/06/statically_compiled_go_programs__always__even_with_cgo__using_musl/
// go build -o q --ldflags '-linkmode external -extldflags "-static"' main/main.go

/*
#cgo LDFLAGS: -lhts -lpthread -lz -lm
#include "stdlib.h"
#include <zlib.h>
#include "htslib/hts.h"
#include "htslib/kstring.h"
#include "htslib/tbx.h"
#include "htslib/bgzf.h"
#include "htslib/sam.h"
#include "htslib/vcf.h"
#include "htslib/faidx.h"
#include "htslib/kfunc.h"

inline hts_itr_t *tabix_itr_queryi(tbx_t *tbx,  int tid, int beg, int end){
   return hts_itr_query((tbx)->idx, (tid), (beg), (end), tbx_readrec);
}
inline int atbx_itr_next(htsFile *fp, tbx_t *tbx, hts_itr_t *iter, kstring_t *data) {
	return tbx_itr_next(fp, tbx, iter, (void *)data);
}

*/
import "C"

import (
	"bytes"
	"fmt"
	"io"
	"runtime"
	"strings"
	"unsafe"

	"github.com/brentp/xopen"
)

type Tbx struct {
	path string
	tbx  *C.tbx_t
	htfs chan *C.htsFile
}

func _close(t *Tbx) {
	if t.tbx != nil {
		C.tbx_destroy(t.tbx)
	}
	if t.htfs != nil {
		for htf := range t.htfs {
			C.hts_close(htf)
		}
	}
}

func New(path string) (*Tbx, error) {
	if !(xopen.Exists(path) && xopen.Exists(path+".tbi")) {
		return nil, fmt.Errorf("need gz file and .tbi for %s", path)
	}

	t := &Tbx{path: path}
	cs, mode := C.CString(t.path), C.char('r')
	defer C.free(unsafe.Pointer(cs))

	t.htfs = make(chan *C.htsFile, 8)
	for i := 0; i < 8; i++ {
		t.htfs <- C.hts_open(cs, &mode)
	}
	t.tbx = C.tbx_index_load(cs)
	runtime.SetFinalizer(t, _close)

	return t, nil
}

func (t *Tbx) Get(chrom string, start int, end int) (io.Reader, error) {
	cchrom := C.CString(chrom)
	ichrom := C.tbx_name2id(t.tbx, cchrom)
	C.free(unsafe.Pointer(cchrom))
	if ichrom == -1 {
		if strings.HasPrefix(chrom, "chr") {
			cchrom = C.CString(chrom[3:])
		} else {
			cchrom = C.CString("chr" + chrom)
		}
		ichrom = C.tbx_name2id(t.tbx, cchrom)
		C.free(unsafe.Pointer(cchrom))
	}
	if ichrom == -1.0 {
		return nil, fmt.Errorf("unknown chromosome: %s", chrom)
	}

	itr := C.tabix_itr_queryi(t.tbx, ichrom, C.int(start), C.int(end))
	kstr := C.kstring_t{}

	l := C.int(10)
	res := make([][]byte, 0, 2)
	// + pull from chans of *C.htsFile
	htf := <-t.htfs
	for l > 0 {
		l := C.atbx_itr_next(htf, t.tbx, itr, &kstr)
		if l < 0 {
			break
		}
		res = append(res, C.GoBytes(unsafe.Pointer(kstr.s), C.int(kstr.l)))
	}
	// unlock
	t.htfs <- htf
	// TODO: keep itr in the Tbx object
	C.hts_itr_destroy(itr)
	C.free(unsafe.Pointer(kstr.s))
	return bytes.NewReader(bytes.Join(res, []byte{'\n'})), nil
}
