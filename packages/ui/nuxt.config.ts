import { fileURLToPath } from 'node:url'

const currentDir = fileURLToPath(new URL('.', import.meta.url))

export default defineNuxtConfig({
  modules: ['@nuxt/icon', '@nuxt/image'],

  css: [`${currentDir}/app/styles/tailwindcss.css`],
})
