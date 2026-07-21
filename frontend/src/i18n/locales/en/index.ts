import landing from './landing'
import common from './common'
import dashboard from './dashboard'
import batchImage from './batchImage'
import admin from './admin'
import misc from './misc'
import custom from './custom'
import { mergeLocale } from '../mergeLocale'

const base = {
  ...landing,
  ...common,
  ...dashboard,
  ...batchImage,
  admin,
  ...misc,
}

export default mergeLocale(base, custom)
