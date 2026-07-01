import withNuxt from './.nuxt/eslint.config.mjs'

export default withNuxt(
  // Generated from the published galgame-wiki OpenAPI spec (openapi-typescript);
  // never hand-edit — the openapi-types.yml drift gate regenerates it.
  { ignores: ['app/shared/types/generated/**'] },
  {
    rules: {
      'no-console': 'off',
      camelcase: 'off',
      'comma-spacing': 'off',
      '@typescript-eslint/no-unused-vars': 'off',
      'vue/multi-word-component-names': 'off',
      'vue/singleline-html-element-content-newline': 'off',
      'vue/html-self-closing': 'off',
      'vue/attributes-order': 'off',
      'vue/no-multiple-template-root': 'off',
      'vue/no-v-html': 'off',
      'import/order': 'off',
      'import/no-named-as-default-member': 'off',
      'arrow-parens': ['error', 'always'],
      'space-before-function-paren': 'off',
      'func-call-spacing': 'off',
      quotes: [
        'error',
        'single',
        { avoidEscape: true, allowTemplateLiterals: true }
      ]
    }
  }
)
