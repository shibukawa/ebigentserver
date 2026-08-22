// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

// https://astro.build/config
export default defineConfig({
	// 公開先に合わせて差し替える（sitemap と canonical URL にだけ効く）。
	// サブパス配信にするなら base も足し、ページ内の絶対リンクを見直すこと。
	site: 'https://example.com',
	integrations: [
		starlight({
			title: 'ebigentserver',
			description:
				'人間・ボット・リプレイを同じ agent として着席させるゲームセッションランタイム。接続方法、ゲームとの接点、ログからの AI 育成を解説する。',
			defaultLocale: 'root',
			locales: {
				root: { label: '日本語', lang: 'ja' },
			},
			social: [
				{
					icon: 'github',
					label: 'GitHub',
					href: 'https://github.com/shibukawa/ebigentserver',
				},
			],
			customCss: ['./src/styles/custom.css'],
			tableOfContents: { minHeadingLevel: 2, maxHeadingLevel: 3 },
			sidebar: [
				{ label: 'ebigentserver とは', slug: 'overview' },
				{
					label: 'チュートリアル',
					items: [
						{ label: 'ここから始める', slug: 'tutorial' },
						{ label: 'step 1 — マウス1つで交互に指す', slug: 'tutorial/step1' },
						{ label: 'step 2 — ロビーと LAN 対戦', slug: 'tutorial/step2' },
					],
				},
				{
					label: '接続方法',
					items: [
						{ label: '接続の選び方', slug: 'connection' },
						{ label: '到達できる組み合わせ', slug: 'connection/deployments' },
						{ label: 'トランスポート', slug: 'connection/transports' },
						{ label: 'シグナリングと着席', slug: 'connection/signaling' },
						{ label: '同期モードと状態配信', slug: 'connection/sync' },
					],
				},
				{
					label: 'ゲームとの接点',
					items: [
						{ label: '接点は4つしかない', slug: 'integration' },
						{ label: 'Simulation インターフェース', slug: 'integration/simulation' },
						{ label: 'Agent インターフェース', slug: 'integration/agent' },
						{ label: '観測と可視性', slug: 'integration/observation' },
						{ label: '越えてはいけない境界', slug: 'integration/boundaries' },
						{ label: 'Ebitengine への統合', slug: 'integration/ebitengine' },
					],
				},
				{
					label: 'ログと AI',
					items: [
						{ label: 'ログが AI になるまで', slug: 'ai' },
						{ label: 'エピソードログの収集', slug: 'ai/episode-log' },
						{ label: '語彙とチップ', slug: 'ai/vocabulary' },
						{ label: '分析ステップと LLM', slug: 'ai/analysis' },
						{ label: '承認・生成・再生成', slug: 'ai/codegen' },
					],
				},
			],
		}),
	],
});
