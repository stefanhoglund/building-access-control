D2FLAGS := --theme 4 --pad 25
D2LAYOUT := elk

DIAGRAMS := context container

FIGURES := $(addprefix docs/figures/,$(addsuffix .svg,$(DIAGRAMS)))

.PHONY: diagrams clean-diagrams

diagrams: $(FIGURES)

docs/figures/%.svg: docs/diagrams/%.d2
	mkdir -p docs/figures
	d2 $(D2FLAGS) --layout $(D2LAYOUT) $< $@

clean-diagrams:
	rm -f docs/figures/*.svg