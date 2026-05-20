FPC        = fpc
SRC        = src
OBJ        = obj
TEST_DIR   = test
FLAGS      = -Mobjfpc -Sh -O2 -Fu$(SRC) -FU$(OBJ)
LDFLAGS    = -k"$(OBJ)/roaring.o" -k"-lc" -k"-dynamic-linker" -k"/lib64/ld-linux-x86-64.so.2"
GRAPH      = JsonToGraph
CSP_EVAL   = CSPEval
SYNTAX_CHK = SyntaxChecker
TEST_ROAR  = TestRoaring
TEST_LSET  = TestLabeledSet
TEST_SEVAL = TestSetEval
TEST_MM      = TestMiniMath
TEST_COMPLEX = TestMiniComplex
TEST_INTERVAL  = TestMiniInterval
FORWARD        = ForwardChain
FUNC_CHK       = FunctionChecker
FWD_CONSIST    = FwdConsistency
BWD_CONSIST    = BwdConsistency
JSON_SINK      = JsonSink
JSON_SOURCE    = JsonSource

MINIMATH_OBJS   = $(OBJ)/minimath_trig.o $(OBJ)/minimath_exp.o $(OBJ)/minimath_util.o $(OBJ)/minimath_interval.o
MINIMATH_KFLAGS = -k"$(OBJ)/minimath_trig.o" -k"$(OBJ)/minimath_exp.o" -k"$(OBJ)/minimath_util.o" -k"$(OBJ)/minimath_interval.o"

GRID_OBJS   = $(OBJ)/minimath_grid.o
GRID_KFLAGS = -k"$(OBJ)/minimath_grid.o"

COMPLEX_OBJS   = $(MINIMATH_OBJS) $(OBJ)/minimath_complex.o
COMPLEX_KFLAGS = $(MINIMATH_KFLAGS) -k"$(OBJ)/minimath_complex.o"

.PHONY: all clean json-graph csp-eval syntax-checker forward-chain fwd-consistency bwd-consistency json-sink json-source func-checker test-roaring test-labeled test-seteval test-minimath test-complex test-interval

$(TEST_DIR):
	mkdir -p $(TEST_DIR)

all: json-graph csp-eval syntax-checker forward-chain fwd-consistency bwd-consistency json-sink json-source func-checker

# ── JsonToGraph — genera grafo de restricciones CSP ──────────────────────────
json-graph: $(SRC)/JsonToGraph.pas $(SRC)/MiniSys.pas $(SRC)/ExpressionAST.pas \
            $(SRC)/PrattParser.pas $(SRC)/MiniJSON.pas | $(OBJ)
	$(FPC) $(FLAGS) $(SRC)/JsonToGraph.pas -o./$(GRAPH)

# ── CSPEval — propagador de dominios CSP ──────────────────────────────────────
csp-eval: $(SRC)/CSPEval.pas $(SRC)/MiniSys.pas $(SRC)/ExpressionAST.pas \
          $(SRC)/PrattParser.pas $(SRC)/MiniJSON.pas $(SRC)/MiniMath.pas \
          $(MINIMATH_OBJS) $(GRID_OBJS) | $(OBJ)
	$(FPC) $(FLAGS) $(MINIMATH_KFLAGS) $(GRID_KFLAGS) $(SRC)/CSPEval.pas -o./$(CSP_EVAL)

# ── JsonSource — fuente de pipeline: SQLite → stdout JSON ─────────────────
json-source: $(SRC)/JsonSource.pas $(SRC)/MiniSys.pas $(OBJ)/libsqlite3.so | $(OBJ)
	$(FPC) $(FLAGS) -Fl$(OBJ) -k"-lsqlite3" $(SRC)/JsonSource.pas -o./$(JSON_SOURCE)

# ── JsonSink — sumidero de pipeline: stdin JSON → SQLite ──────────────────
$(OBJ)/libsqlite3.so:
	ln -sf /usr/lib/x86_64-linux-gnu/libsqlite3.so.0.8.6 $(OBJ)/libsqlite3.so

json-sink: $(SRC)/JsonSink.pas $(SRC)/MiniSys.pas $(SRC)/MiniJSON.pas \
           $(OBJ)/libsqlite3.so | $(OBJ)
	$(FPC) $(FLAGS) -Fl$(OBJ) -k"-lsqlite3" $(SRC)/JsonSink.pas -o./$(JSON_SINK)

# ── BwdConsistency — proyección inversa a través del árbol de expresiones ─
bwd-consistency: $(SRC)/BwdConsistency.pas $(SRC)/MiniSys.pas $(SRC)/ExpressionAST.pas \
                 $(SRC)/PrattParser.pas $(SRC)/MiniJSON.pas $(SRC)/MiniMath.pas \
                 $(MINIMATH_OBJS) $(GRID_OBJS) | $(OBJ)
	$(FPC) $(FLAGS) $(MINIMATH_KFLAGS) $(GRID_KFLAGS) $(SRC)/BwdConsistency.pas -o./$(BWD_CONSIST)

# ── FwdConsistency — verificación de consistencia AC-3 hacia adelante ─────
fwd-consistency: $(SRC)/FwdConsistency.pas $(SRC)/MiniSys.pas $(SRC)/ExpressionAST.pas \
                 $(SRC)/PrattParser.pas $(SRC)/MiniJSON.pas $(SRC)/MiniMath.pas \
                 $(MINIMATH_OBJS) $(GRID_OBJS) | $(OBJ)
	$(FPC) $(FLAGS) $(MINIMATH_KFLAGS) $(GRID_KFLAGS) $(SRC)/FwdConsistency.pas -o./$(FWD_CONSIST)

# ── ForwardChain — encadenamiento hacia adelante con trazado ──────────────
forward-chain: $(SRC)/ForwardChain.pas $(SRC)/MiniSys.pas $(SRC)/ExpressionAST.pas \
               $(SRC)/PrattParser.pas $(SRC)/MiniJSON.pas $(SRC)/MiniMath.pas \
               $(MINIMATH_OBJS) $(GRID_OBJS) | $(OBJ)
	$(FPC) $(FLAGS) $(MINIMATH_KFLAGS) $(GRID_KFLAGS) $(SRC)/ForwardChain.pas -o./$(FORWARD)

# ── FunctionChecker — verifica objetos .o/.so de funciones user-defined ────
func-checker: $(SRC)/FunctionChecker.pas $(SRC)/MiniSys.pas $(SRC)/MiniJSON.pas | $(OBJ)
	$(FPC) $(FLAGS) $(SRC)/FunctionChecker.pas -o./$(FUNC_CHK)

# ── SyntaxChecker ──────────────────────────────────────────────────────────
syntax-checker: $(SRC)/SyntaxChecker.pas $(SRC)/MiniSys.pas $(SRC)/MiniJSON.pas \
                $(SRC)/ExpressionAST.pas $(SRC)/PrattParser.pas | $(OBJ)
	$(FPC) $(FLAGS) $(SRC)/SyntaxChecker.pas -o./$(SYNTAX_CHK)

# ── Objeto C de Roaring ────────────────────────────────────────────────────
$(OBJ)/roaring.o: $(SRC)/roaring.c | $(OBJ)
	gcc -c -O2 -DNDEBUG -D_FORTIFY_SOURCE=0 -fno-builtin -mpopcnt \
	    $(SRC)/roaring.c -o $(OBJ)/roaring.o

# ── Objetos C de MiniMath ─────────────────────────────────────────────────
$(OBJ)/minimath_trig.o: $(SRC)/minimath_trig.c $(SRC)/minimath.h | $(OBJ)
	gcc -c -O2 -std=c99 $(SRC)/minimath_trig.c -o $(OBJ)/minimath_trig.o

$(OBJ)/minimath_exp.o: $(SRC)/minimath_exp.c $(SRC)/minimath.h | $(OBJ)
	gcc -c -O2 -std=c99 $(SRC)/minimath_exp.c -o $(OBJ)/minimath_exp.o

$(OBJ)/minimath_util.o: $(SRC)/minimath_util.c $(SRC)/minimath.h | $(OBJ)
	gcc -c -O2 -std=c99 $(SRC)/minimath_util.c -o $(OBJ)/minimath_util.o

$(OBJ)/minimath_interval.o: $(SRC)/minimath_interval.c $(SRC)/minimath.h | $(OBJ)
	gcc -c -O2 -std=c99 $(SRC)/minimath_interval.c -o $(OBJ)/minimath_interval.o

$(OBJ)/minimath_complex.o: $(SRC)/minimath_complex.c $(SRC)/minimath.h | $(OBJ)
	gcc -c -O2 -std=c99 $(SRC)/minimath_complex.c -o $(OBJ)/minimath_complex.o

$(OBJ)/minimath_grid.o: $(SRC)/minimath_grid.c | $(OBJ)
	gcc -c -O2 -std=c99 $(SRC)/minimath_grid.c -o $(OBJ)/minimath_grid.o

# ── Tests ──────────────────────────────────────────────────────────────────
$(TEST_DIR)/$(TEST_ROAR): $(SRC)/TestRoaring.pas $(SRC)/RoaringBitmap.pas $(OBJ)/roaring.o | $(OBJ) $(TEST_DIR)
	$(FPC) $(FLAGS) $(LDFLAGS) $(SRC)/TestRoaring.pas -o$(TEST_DIR)/$(TEST_ROAR)

test-roaring: $(TEST_DIR)/$(TEST_ROAR)
	./$(TEST_DIR)/$(TEST_ROAR)

$(TEST_DIR)/$(TEST_LSET): $(SRC)/TestLabeledSet.pas $(SRC)/LabeledSet.pas $(OBJ)/roaring.o | $(OBJ) $(TEST_DIR)
	$(FPC) $(FLAGS) $(LDFLAGS) $(SRC)/TestLabeledSet.pas -o$(TEST_DIR)/$(TEST_LSET)

test-labeled: $(TEST_DIR)/$(TEST_LSET)
	./$(TEST_DIR)/$(TEST_LSET)

$(TEST_DIR)/$(TEST_SEVAL): $(SRC)/TestSetEval.pas $(SRC)/ASTEvaluator.pas \
               $(OBJ)/roaring.o $(MINIMATH_OBJS) | $(OBJ) $(TEST_DIR)
	$(FPC) $(FLAGS) $(LDFLAGS) $(MINIMATH_KFLAGS) $(SRC)/TestSetEval.pas -o$(TEST_DIR)/$(TEST_SEVAL)

test-seteval: $(TEST_DIR)/$(TEST_SEVAL)
	./$(TEST_DIR)/$(TEST_SEVAL)

$(TEST_DIR)/$(TEST_MM): $(MINIMATH_OBJS) $(SRC)/TestMiniMath.pas $(SRC)/MiniMath.pas | $(OBJ) $(TEST_DIR)
	$(FPC) $(FLAGS) $(MINIMATH_KFLAGS) $(SRC)/TestMiniMath.pas -o./$(TEST_DIR)/$(TEST_MM)

test-minimath: $(TEST_DIR)/$(TEST_MM)
	./$(TEST_DIR)/$(TEST_MM)

$(TEST_DIR)/$(TEST_COMPLEX): $(COMPLEX_OBJS) $(SRC)/TestMiniComplex.pas $(SRC)/MiniComplex.pas | $(OBJ) $(TEST_DIR)
	$(FPC) $(FLAGS) $(COMPLEX_KFLAGS) $(SRC)/TestMiniComplex.pas -o./$(TEST_DIR)/$(TEST_COMPLEX)

test-complex: $(TEST_DIR)/$(TEST_COMPLEX)
	./$(TEST_DIR)/$(TEST_COMPLEX)

$(TEST_DIR)/$(TEST_INTERVAL): $(MINIMATH_OBJS) $(SRC)/TestMiniInterval.pas $(SRC)/MiniMath.pas | $(OBJ) $(TEST_DIR)
	$(FPC) $(FLAGS) $(MINIMATH_KFLAGS) $(SRC)/TestMiniInterval.pas -o./$(TEST_DIR)/$(TEST_INTERVAL)

test-interval: $(TEST_DIR)/$(TEST_INTERVAL)
	./$(TEST_DIR)/$(TEST_INTERVAL)

# ── Tests del servidor MCP (Python, requiere .venv) ───────────────────────
test-mcp: all
	@echo "Running MCP server tests..."
	@if [ ! -x .venv/bin/python ]; then \
		echo "ERROR: .venv ausente. Ejecute:"; \
		echo "  python3 -m venv .venv && .venv/bin/pip install -r requirements.txt"; \
		exit 1; \
	fi
	@.venv/bin/python -m pytest tests/mcp/ -v

# ── Directorio obj ─────────────────────────────────────────────────────────
$(OBJ):
	mkdir -p $(OBJ)

# ── Limpieza ───────────────────────────────────────────────────────────────
clean:
	rm -f $(GRAPH) $(CSP_EVAL) $(SYNTAX_CHK) $(FORWARD) $(FWD_CONSIST) $(BWD_CONSIST) $(JSON_SINK) $(JSON_SOURCE) $(FUNC_CHK)
	rm -f $(TEST_DIR)/$(TEST_ROAR) $(TEST_DIR)/$(TEST_LSET) $(TEST_DIR)/$(TEST_SEVAL) $(TEST_DIR)/$(TEST_MM) $(TEST_DIR)/$(TEST_COMPLEX) $(TEST_DIR)/$(TEST_INTERVAL)
	rm -f $(OBJ)/*.o $(OBJ)/*.ppu $(OBJ)/*.res link*.res
