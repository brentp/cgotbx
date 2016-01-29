package cgotbx

//http://dominik.honnef.co/posts/2015/06/statically_compiled_go_programs__always__even_with_cgo__using_musl/
// go build -o q --ldflags '-linkmode external -extldflags "-static"' main/main.go

/*
#cgo LDFLAGS: -lhts -lpthread -lz -lm -L/usr/local/lib
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

inline hts_itr_t *tabix_itr_queryi(tbx_t *tbx,  int tid, int beg, int end, int *ret){
   hts_itr_t *h = hts_itr_query((tbx)->idx, (tid), (beg), (end), tbx_readrec);
   if(h->n_off == 0) {
	   hts_itr_destroy(h);
	   *ret = -1;
   }
   return h;
}

inline int tbx_itr_next5(htsFile *fp, tbx_t *tbx, hts_itr_t *iter, kstring_t *data1, kstring_t *data2) {
	int t1 = tbx_itr_next(fp, tbx, iter, (void *)data1);
	if(t1<=0){ hts_itr_destroy(iter); return -1; }
	kputc('\n', data1);

	int t2 = tbx_itr_next(fp, tbx, iter, (void *)data2);
	if(t2<=0){ hts_itr_destroy(iter); return 0; }
	kputsn(data2->s, data2->l, data1);
	kputc('\n', data1);

	int t3 = tbx_itr_next(fp, tbx, iter, (void *)data2);
	if(t3<=0){ hts_itr_destroy(iter); return 0; }
	kputsn(data2->s, data2->l, data1);
	kputc('\n', data1);


	int t4 = tbx_itr_next(fp, tbx, iter, (void *)data2);
	if(t4<=0){ hts_itr_destroy(iter); return 0; }
	kputsn(data2->s, data2->l, data1);
	kputc('\n', data1);

	int t5 = tbx_itr_next(fp, tbx, iter, (void *)data2);
	if(t5<=0){ hts_itr_destroy(iter); return 0; }
	kputsn(data2->s, data2->l, data1);
	kputc('\n', data1);

	return 1;
}

*/
import "C"

import (
	"fmt"
	"io"
	"log"
	"runtime"
	"strings"
	"sync"
	"unsafe"

	"github.com/brentp/xopen"
)

type Tbx struct {
	path   string
	tbx    *C.tbx_t
	htfs   chan *C.htsFile
	kCache chan C.kstring_t

	mu         sync.RWMutex
	chromCache map[string]C.int
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
	if !(xopen.Exists(path) && (xopen.Exists(path+".tbi") || xopen.Exists(path+".csi"))) {
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

	t.kCache = make(chan C.kstring_t, size*2)
	for i := 0; i < cap(t.kCache); i++ {
		t.kCache <- C.kstring_t{}
	}
	t.chromCache = make(map[string]C.int)

	if xopen.Exists(path + ".csi") {
		csi := C.CString(path + ".csi")
		defer C.free(unsafe.Pointer(csi))
		t.tbx = C.tbx_index_load2(cs, csi)
	} else {
		t.tbx = C.tbx_index_load(cs)
	}
	runtime.SetFinalizer(t, _close)

	return t, nil
}

func (t *Tbx) getChrom(chrom string) C.int {

	t.mu.RLock()
	if i, ok := t.chromCache[chrom]; ok {
		t.mu.RUnlock()
		return i
	}
	t.mu.RUnlock()
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
	t.mu.Lock()
	t.chromCache[chrom] = ichrom
	t.mu.Unlock()
	return ichrom
}

func (t *Tbx) Get(chrom string, start int, end int) (io.Reader, error) {
	ichrom := t.getChrom(chrom)
	if ichrom == -1.0 {
		log.Printf("chromosome: %s not found in %s \n", chrom, t.path)
		return strings.NewReader(""), nil
	}
	ret := C.int(1)

	itr := C.tabix_itr_queryi(t.tbx, ichrom, C.int(start), C.int(end), &ret)
	if ret < 0 {
		return strings.NewReader(""), nil
	}

	p := newPipe(1)
	go func() {
		l := C.int(10)
		kstr1, kstr2 := <-t.kCache, <-t.kCache
		htf := <-t.htfs
		for l > 0 {
			// < 0 means no data
			// == 0 means we got data, but end after this
			l = C.tbx_itr_next5(htf, t.tbx, itr, &kstr1, &kstr2)
			if l < 0 {
				break
			}
			res := C.GoBytes(unsafe.Pointer(kstr1.s), C.int(kstr1.l))
			_, err := p.Write(res)
			if err != nil {
				log.Fatal(err)
			}
		}
		close(p.ch)
		// unlock
		t.htfs <- htf
		t.kCache <- kstr1
		t.kCache <- kstr2
	}()
	return p, nil
}
