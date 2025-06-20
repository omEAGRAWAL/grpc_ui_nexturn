import { useState } from 'react'
import reactLogo from './assets/react.svg'
import viteLogo from '/vite.svg'
import './App.css'
import GRPCUIFrontend from './GRPC_UI'

function App() {
  const [count, setCount] = useState(0)

  return (
    <>
      <GRPCUIFrontend />
    </>
  )
}

export default App
