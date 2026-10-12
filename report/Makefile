# Build the final report PDF.
#
# Usage:
#   make            # build the PDF (3 passes, so the TOC/refs settle)
#   make diagrams   # re-render figures/*.png from docs/diagrams/*.d2
#                    # (requires Google Chrome; see scripts/render-diagrams-png.sh)
#   make clean      # remove LaTeX intermediate files
#   make distclean  # also remove the PDF

PDFLATEX := pdflatex -interaction=nonstopmode -halt-on-error
DOC      := final-report

.PHONY: all diagrams clean distclean
all: $(DOC).pdf
	@echo "==> $(DOC).pdf ready"

$(DOC).pdf: $(DOC).tex
	$(PDFLATEX) $(DOC).tex
	$(PDFLATEX) $(DOC).tex
	$(PDFLATEX) $(DOC).tex
	@rm -f $(DOC).aux $(DOC).log $(DOC).out $(DOC).toc

diagrams:
	../scripts/render-diagrams-png.sh

clean:
	rm -f $(DOC).aux $(DOC).log $(DOC).out $(DOC).toc

distclean: clean
	rm -f $(DOC).pdf
