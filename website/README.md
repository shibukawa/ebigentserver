# ebigentserver ドキュメントサイト

[Astro](https://astro.build/) + [Starlight](https://starlight.astro.build/) で作った
ebigentserver のドキュメント。日本語。

## 構成

```
src/
├── content/docs/
│   ├── index.mdx            ランディング
│   ├── overview.mdx         全体像（agent 抽象、層構造、直交する2軸）
│   ├── connection/          接続方法（トポロジ・トランスポート・シグナリング・同期）
│   ├── integration/         ゲームとの接点（Game / Agent / 観測 / 境界）
│   └── ai/                  ログの収集、蒸留パイプライン、コード生成
└── styles/custom.css        テーマ対応の図版スタイル
```

## 開発

```bash
npm install
```

```bash
npm run dev
```

```bash
npm run build
```

`dist/` に静的サイトが出る。

## 図版について

図は inline SVG で、`.diagram` の CSS クラス経由で Starlight のカラートークンを参照する。
ライト／ダークの両テーマで自動的に色が入れ替わるので、SVG に色をハードコードしないこと。
狭い画面では図の中だけが横スクロールし、ページ本体は横に流れない。

## 公開先

`astro.config.mjs` の `site` は `https://example.com` のままなので、公開先に合わせて
差し替える（sitemap と canonical URL にだけ効く）。サブパス配信にする場合は `base` も足し、
ページ内の絶対リンク（`/connection/` など）を見直す。
