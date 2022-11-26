import { series } from 'gulp'
import { boil } from './boil'
import { cleanTemp } from './clean'
import { compile } from './compile'
import { generate } from './generate'
import { patch } from './patch'

export * from './assets'
export * from './boil'
export * from './clean'
export * from './compile'
export * from './compileShell'
export * from './generate'
export * from './patch'
export * from './userscript'

export const build = series(generate, compile, patch, boil, cleanTemp)
