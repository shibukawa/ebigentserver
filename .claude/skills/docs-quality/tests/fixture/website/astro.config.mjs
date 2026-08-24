import starlight from '@astrojs/starlight';
export default {
  integrations: [
    starlight({
      customCss: ['./src/styles/custom.css'],
      social: [{ icon: 'github', href: 'https://example.com' }],
      sidebar: [
        { label: 'step 1', slug: 'tutorial/step1' },
        { label: 'step 2', slug: 'tutorial/step2' },
        { label: '用語', slug: 'architecture/terms' },
        { label: '概要', slug: 'overview' },
        { label: '無い', slug: 'tutorial/step9' },
      ],
    }),
  ],
};
