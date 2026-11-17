import { useEffect, useRef } from "react";
import { basicSetup } from "codemirror";
import { StandardSQL, sql } from "@codemirror/lang-sql";
import { HighlightStyle, syntaxHighlighting } from "@codemirror/language";
import { EditorState, Prec } from "@codemirror/state";
import { EditorView, keymap } from "@codemirror/view";
import { tags } from "@lezer/highlight";

const editorTheme = EditorView.theme({
  "&": {
    backgroundColor: "transparent",
    color: "var(--ql-ink)",
    fontSize: "12px",
    height: "100%",
  },
  ".cm-scroller": {
    fontFamily: "var(--font-mono)",
    lineHeight: "1.65",
    overflow: "auto",
  },
  ".cm-content": {
    padding: "10px 0",
    caretColor: "var(--ql-accent)",
  },
  ".cm-gutters": {
    backgroundColor: "transparent",
    color: "var(--ql-ink-faint)",
    border: "none",
  },
  ".cm-lineNumbers .cm-gutterElement": {
    padding: "0 8px 0 12px",
  },
  ".cm-activeLine": {
    backgroundColor: "color-mix(in srgb, var(--ql-hover) 55%, transparent)",
  },
  ".cm-activeLineGutter": {
    backgroundColor: "transparent",
  },
  "&.cm-focused": {
    outline: "none",
  },
  ".cm-selectionBackground, ::selection": {
    backgroundColor: "color-mix(in srgb, var(--ql-accent) 25%, transparent)",
  },
  ".cm-cursor": {
    borderLeftColor: "var(--ql-accent)",
  },
});

const highlightStyle = HighlightStyle.define([
  { tag: tags.keyword, color: "var(--ql-accent)" },
  { tag: [tags.string, tags.special(tags.string)], color: "var(--ql-success)" },
  { tag: [tags.number, tags.bool, tags.null], color: "var(--ql-warning)" },
  {
    tag: [tags.comment, tags.lineComment, tags.blockComment],
    color: "var(--ql-ink-faint)",
    fontStyle: "italic",
  },
  { tag: [tags.operator, tags.punctuation], color: "var(--ql-ink-muted)" },
  { tag: [tags.variableName, tags.propertyName], color: "var(--ql-ink)" },
  { tag: tags.typeName, color: "var(--ql-op-project)" },
]);

type SqlEditorProps = {
  value: string;
  onChange: (value: string) => void;
  onRun: () => void;
  visible?: boolean;
};

export function SqlEditor({ value, onChange, onRun, visible = true }: SqlEditorProps) {
  const host = useRef<HTMLDivElement>(null);
  const view = useRef<EditorView | null>(null);
  const initialValue = useRef(value);
  const onChangeRef = useRef(onChange);
  const onRunRef = useRef(onRun);

  useEffect(() => {
    onChangeRef.current = onChange;
  }, [onChange]);

  useEffect(() => {
    onRunRef.current = onRun;
  }, [onRun]);

  useEffect(() => {
    if (!host.current) {
      return;
    }

    const instance = new EditorView({
      state: EditorState.create({
        doc: initialValue.current,
        extensions: [
          basicSetup,
          sql({ dialect: StandardSQL }),
          syntaxHighlighting(highlightStyle),
          editorTheme,
          EditorView.lineWrapping,
          EditorView.contentAttributes.of({ "aria-label": "SQL editor" }),
          Prec.high(
            keymap.of([
              {
                key: "Mod-Enter",
                run: () => {
                  onRunRef.current();
                  return true;
                },
              },
            ]),
          ),
          EditorView.updateListener.of((update) => {
            if (update.docChanged) {
              onChangeRef.current(update.state.doc.toString());
            }
          }),
        ],
      }),
      parent: host.current,
    });

    view.current = instance;
    return () => {
      instance.destroy();
      view.current = null;
    };
  }, []);

  useEffect(() => {
    const instance = view.current;
    if (!instance) {
      return;
    }

    const current = instance.state.doc.toString();
    if (current !== value) {
      instance.dispatch({
        changes: { from: 0, to: current.length, insert: value },
      });
    }
  }, [value]);

  useEffect(() => {
    if (visible) {
      view.current?.requestMeasure();
    }
  }, [visible]);

  return <div ref={host} className="h-full min-h-0 overflow-hidden" />;
}
