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

inline int tbx_itr_next5(htsFile *fp, tbx_t *tbx, hts_itr_t *iter, kstring_t *data1, kstring_t *data2, kstring_t *data3, kstring_t *data4, kstring_t *data5) {
	int t1 = tbx_itr_next(fp, tbx, iter, (void *)data1);
	if(t1<=0){ return 0; }
	kputc('\n', data1);

	int t2 = tbx_itr_next(fp, tbx, iter, (void *)data2);
	if(t2<=0){ return 1; }
	kputsn(data2->s, data2->l, data1);
	kputc('\n', data1);

	int t3 = tbx_itr_next(fp, tbx, iter, (void *)data3);
	if(t3<=0){ return 2; }
	kputsn(data3->s, data3->l, data1);
	kputc('\n', data1);


	int t4 = tbx_itr_next(fp, tbx, iter, (void *)data4);
	if(t4<=0){ return 3; }
	kputsn(data4->s, data4->l, data1);
	kputc('\n', data1);

	int t5 = tbx_itr_next(fp, tbx, iter, (void *)data5);
	if(t5<=0){ return 4; }
	kputsn(data5->s, data5->l, data1);
	kputc('\n', data1);

	return 5;
}

*/
import "C"

import (
	"fmt"
	"io"
	"log"
	"runtime"
	"strings"
	"unsafe"

	"github.com/brentp/xopen"
)

type Tbx struct {
	path   string
	tbx    *C.tbx_t
	htfs   chan *C.htsFile
	kCache chan C.kstring_t
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
	if t.kCache != nil {
		for kstr := range t.kCache {
			C.free(unsafe.Pointer(kstr.s))
		}
	}
	close(t.htfs)
	close(t.kCache)
}

func New(path string, n ...int) (*Tbx, error) {
	if !(xopen.Exists(path) && xopen.Exists(path+".tbi")) {
		return nil, fmt.Errorf("need gz file and .tbi for %s", path)
	}
	size := 20
	if len(n) > 0 {
		size = n[0]
	}

	t := &Tbx{path: path}
	cs, mode := C.CString(t.path), C.char('r')
	defer C.free(unsafe.Pointer(cs))

	t.htfs = make(chan *C.htsFile, size)
	for i := 0; i < cap(t.htfs); i++ {
		t.htfs <- C.hts_open(cs, &mode)
	}

	t.kCache = make(chan C.kstring_t, size*5)
	for i := 0; i < cap(t.kCache); i++ {
		t.kCache <- C.kstring_t{}
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
		log.Printf("chromosome: %s not found in %s \n", chrom, t.path)
		return strings.NewReader(""), nil
	}

	itr := C.tabix_itr_queryi(t.tbx, ichrom, C.int(start), C.int(end))

	l := C.int(10)
	p := newPipe(make([]byte, 0, 32), 2) // TODO keep a cache of pipes?
	// + pull from chans of *C.htsFile
	n, times := 0, 0
	go func() {
		var kstr1, kstr2, kstr3, kstr4, kstr5 = <-t.kCache, <-t.kCache, <-t.kCache, <-t.kCache, <-t.kCache
		//var kstr1, kstr2, kstr3 = C.kstring_t{}, C.kstring_t{}, C.kstring_t{}
		htf := <-t.htfs
		for l > 0 {
			l := C.tbx_itr_next5(htf, t.tbx, itr, &kstr1, &kstr2, &kstr3, &kstr4, &kstr5)
			if l <= 0 {
				break
			}
			n += int(l)
			times += 1
			res := C.GoBytes(unsafe.Pointer(kstr1.s), C.int(kstr1.l))
			_, err := p.Write(res)
			if err != nil {
				log.Fatal(err)
			}
		}
		//log.Println(n, times)
		close(p.ch)
		// unlock
		t.htfs <- htf
		C.hts_itr_destroy(itr)
		t.kCache <- kstr1
		t.kCache <- kstr2
		t.kCache <- kstr3
		t.kCache <- kstr4
		t.kCache <- kstr5
	}()
	return p, nil
}
