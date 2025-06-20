import React, { useState, useEffect, useRef } from 'react';
import { Upload, Send, List, Wifi, WifiOff, Play, Square, MessageCircle, ArrowUpDown, ArrowDown, ArrowUp, Zap, Server, Globe, Activity, Sun, Moon } from 'lucide-react';
//
// const API_BASE = 'http://localhost:8081/api';
// const WS_BASE = 'ws://localhost:8081/grpc/ws/stream';
const API_BASE = '/api';
const WS_BASE  = ((location.protocol === 'https:') ? 'wss://' : 'ws://') + location.host + '/grpc/ws/stream';


export default function GRPCUIFrontend() {
  const [darkMode, setDarkMode] = useState(true);
  const [protoFile, setProtoFile] = useState(null);
  const [uploadStatus, setUploadStatus] = useState('');
  const [services, setServices] = useState({});
  const [selectedService, setSelectedService] = useState('');
  const [selectedMethod, setSelectedMethod] = useState('');
  const [target, setTarget] = useState('localhost:50051');
  const [requestData, setRequestData] = useState('{}');
  const [response, setResponse] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [activeTab, setActiveTab] = useState('unary');

  // WebSocket streaming states
  const [wsConnection, setWsConnection] = useState(null);
  const [streamMessages, setStreamMessages] = useState([]);
  const [streamMode, setStreamMode] = useState('server');
  const [isStreaming, setIsStreaming] = useState(false);
  const [streamInput, setStreamInput] = useState('{}');

  const wsRef = useRef(null);
  const messagesEndRef = useRef(null);

  // --- metadata ---
  const [metadataJson, setMetadataJson] = useState('{}');  // free‑form JSON

// --- auth ---
  const [authType, setAuthType]       = useState('none');  // 'none' | 'bearer' | 'basic'
  const [authToken, setAuthToken]     = useState('');
  const [authUser,  setAuthUser]      = useState('');
  const [authPass,  setAuthPass]      = useState('');


  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  };

  useEffect(() => {
    scrollToBottom();
  }, [streamMessages]);

  const handleFileUpload = async () => {
    if (!protoFile) {
      setUploadStatus('Please select a .proto file');
      return;
    }

    const formData = new FormData();
    formData.append('proto', protoFile);

    try {
      setIsLoading(true);
      const response = await fetch(`${API_BASE}/upload/proto`, {
        method: 'POST',
        body: formData,
      });

      const result = await response.json();
      if (response.ok) {
        setUploadStatus('✅ ' + result.message);
        loadServices();
      } else {
        setUploadStatus('❌ ' + result.error);
      }
    } catch (error) {
      setUploadStatus('❌ Upload failed: ' + error.message);
    } finally {
      setIsLoading(false);
    }
  };

  const loadServices = async () => {
    try {
      const response = await fetch(`${API_BASE}/listServices`);
      const result = await response.json();
      if (response.ok) {
        setServices(result);
        setSelectedService('');
        setSelectedMethod('');
      }
    } catch (error) {
      console.error('Failed to load services:', error);
    }
  };
  //
  // const startWebSocketStream = () => {
  //   if (!selectedService || !selectedMethod) {
  //     setStreamMessages(prev => [...prev, { type: 'error', content: 'Please select a service and method' }]);
  //     return;
  //   }
  //
  //   try {
  //     const ws = new WebSocket(WS_BASE);
  //     wsRef.current = ws;
  //
  //     ws.onopen = () => {
  //       setWsConnection(ws);
  //       setIsStreaming(true);
  //       setStreamMessages(prev => [...prev, { type: 'system', content: `Connected to WebSocket. Mode: ${streamMode}` }]);
  //
  //       // Send initialization message
  //       const initMessage = {
  //         target,
  //         service: selectedService,
  //         method: selectedMethod,
  //         mode: ""
  //       };
  //       ws.send(JSON.stringify(initMessage));
  //     };
  //
  //     ws.onmessage = (event) => {
  //       try {
  //         const data = JSON.parse(event.data);
  //         setStreamMessages(prev => [...prev, {
  //           type: 'response',
  //           content: JSON.stringify(data, null, 2),
  //           timestamp: new Date().toLocaleTimeString()
  //         }]);
  //       } catch {
  //         setStreamMessages(prev => [...prev, {
  //           type: 'response',
  //           content: event.data,
  //           timestamp: new Date().toLocaleTimeString()
  //         }]);
  //       }
  //     };
  //
  //     ws.onerror = (error) => {
  //       setStreamMessages(prev => [...prev, { type: 'error', content: 'WebSocket error occurred' }]);
  //     };
  //
  //     ws.onclose = () => {
  //       setWsConnection(null);
  //       setIsStreaming(false);
  //       setStreamMessages(prev => [...prev, { type: 'system', content: 'WebSocket connection closed' }]);
  //     };
  //
  //   } catch (error) {
  //     setStreamMessages(prev => [...prev, { type: 'error', content: 'Failed to connect: ' + error.message }]);
  //   }
  // };
  const startWebSocketStream = () => {
    if (!selectedService || !selectedMethod) {
      setStreamMessages(prev => [
        ...prev,
        { type: 'error', content: 'Please select a service and method' }
      ]);
      return;
    }

    try {
      const ws = new WebSocket(WS_BASE);
      wsRef.current = ws;

      ws.onopen = () => {
        setWsConnection(ws);
        setIsStreaming(true);
        setStreamMessages(prev => [
          ...prev,
          { type: 'system', content: `Connected to WebSocket. Mode` }
        ]);

        // ---- build init message with metadata + auth ----
        const md =
            metadataJson.trim() !== '' ? JSON.parse(metadataJson) : undefined;

        const initMessage = {
          target,
          service: selectedService,
          method: selectedMethod,
          mode:"", // let server infer if you want: ""
          metadata: md,
          auth:
              authType === 'bearer'
                  ? { type: 'bearer', token: authToken }
                  : authType === 'basic'
                      ? { type: 'basic', username: authUser, password: authPass }
                      : undefined
        };

        ws.send(JSON.stringify(initMessage));
      };

      ws.onmessage = event => {
        try {
          const data = JSON.parse(event.data);
          setStreamMessages(prev => [
            ...prev,
            {
              type: 'response',
              content: JSON.stringify(data, null, 2),
              timestamp: new Date().toLocaleTimeString()
            }
          ]);
        } catch {
          setStreamMessages(prev => [
            ...prev,
            {
              type: 'response',
              content: event.data,
              timestamp: new Date().toLocaleTimeString()
            }
          ]);
        }
      };

      ws.onerror = () => {
        setStreamMessages(prev => [
          ...prev,
          { type: 'error', content: 'WebSocket error occurred' }
        ]);
      };

      ws.onclose = () => {
        setWsConnection(null);
        setIsStreaming(false);
        setStreamMessages(prev => [
          ...prev,
          { type: 'system', content: 'WebSocket connection closed' }
        ]);
      };
    } catch (error) {
      setStreamMessages(prev => [
        ...prev,
        { type: 'error', content: 'Failed to connect: ' + error.message }
      ]);
    }
  };

  const stopWebSocketStream = () => {
    if (wsRef.current) {
      wsRef.current.close();
    }
  };

  const sendStreamMessage = () => {
    if (wsConnection && streamInput.trim()) {
      try {
        const data = JSON.parse(streamInput);
        wsConnection.send(JSON.stringify(data));
        setStreamMessages(prev => [...prev, {
          type: 'sent',
          content: JSON.stringify(data, null, 2),
          timestamp: new Date().toLocaleTimeString()
        }]);
        setStreamInput('{}');
      } catch {
        setStreamMessages(prev => [...prev, { type: 'error', content: 'Invalid JSON in stream input' }]);
      }
    }
  };

  const sendEndSignal = () => {
    if (wsConnection) {
      wsConnection.send('__END__');
      setStreamMessages(prev => [...prev, { type: 'system', content: 'End signal sent' }]);
    }
  };

  const clearStreamMessages = () => {
    setStreamMessages([]);
  };

  const toggleTheme = () => {
    setDarkMode(!darkMode);
  };



  // Theme classes
  const themeClasses = {
    background: darkMode
        ? "min-h-screen bg-gradient-to-br from-black via-gray-900 to-gray-800"
        : "min-h-screen bg-gradient-to-br from-gray-50 via-white to-gray-100",

    card: darkMode
        ? "bg-gradient-to-br from-gray-900/90 via-black/70 to-gray-800/80 backdrop-blur-xl border border-[#00bfa6]/20 shadow-2xl shadow-[#00bfa6]/10"
        : "bg-white/95 backdrop-blur-xl border border-[#00bfa6]/30 shadow-2xl shadow-[#00bfa6]/20",

    primaryButton: darkMode
        ? "bg-gradient-to-r from-[#00bfa6] via-[#00bfa6]/90 to-[#00695c] hover:from-[#00bfa6]/90 hover:via-[#00bfa6] hover:to-[#00bfa6]/80 text-white shadow-lg shadow-[#00bfa6]/30 hover:shadow-[#00bfa6]/50"
        : "bg-gradient-to-r from-[#00bfa6] to-[#00897b] hover:from-[#00897b] hover:to-[#00695c] text-white shadow-lg shadow-[#00bfa6]/40",

    secondaryButton: darkMode
        ? "bg-gradient-to-r from-gray-800/60 to-gray-700/60 border border-[#00bfa6]/30 text-gray-300 hover:border-[#00bfa6]/50 hover:bg-gradient-to-r hover:from-gray-700/70 hover:to-gray-600/70"
        : "bg-gradient-to-r from-gray-100 to-gray-200 border border-[#00bfa6]/40 text-gray-700 hover:border-[#00bfa6]/60 hover:bg-gradient-to-r hover:from-gray-50 hover:to-gray-100",

    input: darkMode
        ? "bg-black/50 border border-[#00bfa6]/30 text-white focus:border-[#00bfa6] focus:ring-[#00bfa6]/30 focus:shadow-lg focus:shadow-[#00bfa6]/20"
        : "bg-white/80 border border-[#00bfa6]/40 text-gray-900 focus:border-[#00bfa6] focus:ring-[#00bfa6]/30 focus:shadow-lg focus:shadow-[#00bfa6]/20",

    text: {
      primary: darkMode ? "text-white" : "text-gray-900",
      secondary: darkMode ? "text-gray-300" : "text-gray-600",
      accent: "text-[#00bfa6]"
    },

    accent: "bg-gradient-to-r from-[#00bfa6] to-[#00897b]"
  };

  return (
      <div className={themeClasses.background}>
        {/* Enhanced floating elements with #00bfa6 */}
        <div className="absolute inset-0 overflow-hidden pointer-events-none">
          <div className="absolute top-20 left-20 w-40 h-40 bg-gradient-to-br from-[#00bfa6]/20 via-[#00bfa6]/10 to-transparent rounded-full animate-pulse blur-sm"></div>
          <div className="absolute top-60 right-32 w-32 h-32 bg-[#00bfa6]/30 rotate-45 animate-pulse delay-1000 rounded-lg"></div>
          <div className="absolute bottom-32 left-1/3 w-48 h-48 bg-gradient-to-tr from-[#00bfa6]/15 via-[#00bfa6]/5 to-transparent rounded-full animate-pulse delay-500"></div>
          <div className="absolute top-1/3 right-20 w-20 h-80 bg-gradient-to-b from-[#00bfa6]/20 via-[#00bfa6]/10 to-transparent rotate-12 rounded-full"></div>
          <div className="absolute top-40 left-1/2 w-6 h-6 bg-[#00bfa6] rounded-full animate-ping opacity-20"></div>
          <div className="absolute bottom-40 right-1/4 w-4 h-4 bg-[#00bfa6] rounded-full animate-ping opacity-30 delay-700"></div>
        </div>

        <div className="relative z-10 container mx-auto p-6 max-w-7xl">
          {/* Header with Theme Toggle */}
          <div className="flex justify-between items-center mb-12">
            <div className="text-center flex-1">
              <div className="inline-flex items-center gap-4 mb-6">
                <div className={`p-4 ${themeClasses.accent} rounded-2xl shadow-lg`}>
                  <Zap className="w-10 h-10 text-white" />
                </div>
                <div>
                  <h1 className={`text-6xl font-black ${themeClasses.text.primary} mb-2 relative`}>
                    <span className="relative z-10">gRPC</span>
                    <span className="text-[#00bfa6] relative z-10 drop-shadow-lg">Studio</span>
                    <div className="absolute -inset-2 bg-gradient-to-r from-[#00bfa6]/20 via-[#00bfa6]/10 to-transparent blur-xl opacity-60 animate-pulse"></div>
                  </h1>
                  <div className="h-2 w-40 bg-gradient-to-r from-[#00bfa6] via-[#00bfa6]/80 to-[#00bfa6]/40 mx-auto rounded-full shadow-lg shadow-[#00bfa6]/30"></div>
                </div>
              </div>
              <p className={`text-xl ${themeClasses.text.secondary} max-w-2xl mx-auto`}>
                Professional gRPC testing suite with real-time streaming capabilities
              </p>
            </div>

            <button
                onClick={toggleTheme}
                className={`p-4 rounded-2xl transition-all duration-300 hover:scale-110 ${themeClasses.secondaryButton} group relative overflow-hidden`}
            >
              <div className="absolute inset-0 bg-gradient-to-r from-[#00bfa6]/10 to-[#00bfa6]/20 opacity-0 group-hover:opacity-100 transition-opacity duration-300"></div>
              <div className="relative z-10">
                {darkMode ? <Sun className="w-6 h-6" /> : <Moon className="w-6 h-6" />}
              </div>
            </button>
          </div>

          {/* Proto Upload Section */}
          <div className={`${themeClasses.card} rounded-3xl p-8 mb-8 hover:shadow-2xl transition-all duration-300`}>
            <div className="flex items-center gap-4 mb-8">
              <div className={`p-3 ${themeClasses.accent} rounded-xl`}>
                <Upload className="w-7 h-7 text-white" />
              </div>
              <div>
                <h2 className={`text-3xl font-bold ${themeClasses.text.primary}`}>Protocol Buffer</h2>
                <div className="h-1 w-24 bg-gradient-to-r from-[#00bfa6] via-[#00bfa6]/70 to-[#00bfa6]/40 mt-2 rounded-full shadow-sm shadow-[#00bfa6]/40"></div>
              </div>
            </div>

            <div className="grid grid-cols-1 lg:grid-cols-4 gap-6 items-end">
              <div className="lg:col-span-3">
                <label className={`block text-sm font-semibold ${themeClasses.text.secondary} mb-4`}>
                  Select .proto file
                </label>
                <div className="relative group">
                  <input
                      type="file"
                      accept=".proto"
                      onChange={(e) => setProtoFile(e.target.files[0])}
                      className={`w-full p-5 ${themeClasses.input} rounded-2xl backdrop-blur-sm transition-all duration-200 file:mr-4 file:py-3 file:px-6 file:rounded-xl file:border-0 file:bg-gradient-to-r file:from-[#00bfa6] file:to-[#00897b] file:text-white file:font-semibold hover:file:from-[#00897b] hover:file:to-[#00695c] file:transition-all file:duration-200 file:shadow-lg file:shadow-[#00bfa6]/30`}
                  />
                  <div className="absolute inset-0 rounded-2xl bg-gradient-to-r from-[#00bfa6]/0 via-[#00bfa6]/0 to-[#00bfa6]/0 group-hover:from-[#00bfa6]/5 group-hover:via-[#00bfa6]/10 group-hover:to-[#00bfa6]/5 transition-all duration-300 pointer-events-none"></div>
                  <div className="absolute -inset-0.5 bg-gradient-to-r from-[#00bfa6]/20 to-[#00bfa6]/40 rounded-2xl opacity-0 group-hover:opacity-100 transition-opacity duration-300 -z-10 blur-sm"></div>
                </div>
              </div>
              <button
                  onClick={handleFileUpload}
                  disabled={isLoading || !protoFile}
                  className={`px-8 py-5 ${themeClasses.primaryButton} disabled:opacity-50 disabled:cursor-not-allowed rounded-2xl font-bold transition-all duration-200 transform hover:scale-105 disabled:hover:scale-100 relative overflow-hidden group`}
              >
                <div className="absolute inset-0 bg-gradient-to-r from-[#00bfa6]/20 to-[#00bfa6]/40 opacity-0 group-hover:opacity-100 transition-opacity duration-300"></div>
                <div className="relative z-10">
                  {isLoading ? (
                      <div className="flex items-center gap-3">
                        <div className="w-5 h-5 border-2 border-current/30 border-t-current rounded-full animate-spin"></div>
                        Uploading...
                      </div>
                  ) : (
                      'Upload Proto'
                  )}
                </div>
              </button>
            </div>

            {uploadStatus && (
                <div className={`mt-8 p-5 ${darkMode ? 'bg-gray-900/60' : 'bg-gray-100'} rounded-2xl border ${darkMode ? 'border-gray-700/50' : 'border-gray-200'}`}>
                  <div className="text-sm font-mono">{uploadStatus}</div>
                </div>
            )}
          </div>

          {/* Services Configuration */}
          <div className={`${themeClasses.card} rounded-3xl p-8 mb-8`}>
            <div className="flex items-center gap-4 mb-8">
              <div className={`p-3 ${themeClasses.accent} rounded-xl`}>
                <Server className="w-7 h-7 text-white" />
              </div>
              <div>
                <h2 className={`text-3xl font-bold ${themeClasses.text.primary}`}>Service Configuration</h2>
                <div className="h-1 w-28 bg-gradient-to-r from-[#00bfa6] via-[#00bfa6]/70 to-[#00bfa6]/40 mt-2 rounded-full shadow-sm shadow-[#00bfa6]/40"></div>
              </div>
            </div>

            <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
              <div className="space-y-4">
                <label className={`block text-sm font-semibold ${themeClasses.text.secondary}`}>Service</label>
                <select
                    value={selectedService}
                    onChange={(e) => {
                      setSelectedService(e.target.value);
                      setSelectedMethod('');
                    }}
                    className={`w-full p-5 ${themeClasses.input} rounded-2xl backdrop-blur-sm focus:ring-2 transition-all duration-200`}
                >
                  <option value="">Choose a service</option>
                  {Object.keys(services).map(service => (
                      <option key={service} value={service}>{service}</option>
                  ))}
                </select>
              </div>

              <div className="space-y-4">
                <label className={`block text-sm font-semibold ${themeClasses.text.secondary}`}>Method</label>
                <select
                    value={selectedMethod}
                    onChange={(e) => setSelectedMethod(e.target.value)}
                    disabled={!selectedService}
                    className={`w-full p-5 ${themeClasses.input} rounded-2xl backdrop-blur-sm disabled:opacity-50 disabled:cursor-not-allowed focus:ring-2 transition-all duration-200`}
                >
                  <option value="">Choose a method</option>
                  {selectedService && services[selectedService]?.map(method => (
                      <option key={method} value={method}>{method}</option>
                  ))}
                </select>
              </div>

              <div className="space-y-4">
                <label className={`block text-sm font-semibold ${themeClasses.text.secondary}`}>Target Server</label>
                <div className="relative">
                  <Globe className={`absolute left-4 top-1/2 transform -translate-y-1/2 w-6 h-6 ${themeClasses.text.secondary}`} />
                  <input
                      type="text"
                      value={target}
                      onChange={(e) => setTarget(e.target.value)}
                      placeholder="localhost:50051"
                      className={`w-full pl-14 pr-5 py-5 ${themeClasses.input} rounded-2xl backdrop-blur-sm focus:ring-2 transition-all duration-200`}
                  />
                </div>
              </div>
            </div>
          </div>

          {/* Streaming Interface */}
          <div className={`${themeClasses.card} rounded-3xl overflow-hidden`}>
            <div className="p-8">
              <div className="flex items-center gap-4 mb-8">
                <div className={`p-3 ${themeClasses.accent} rounded-xl`}>
                  <Activity className="w-7 h-7 text-white" />
                </div>
                <div>
                  <h3 className={`text-3xl font-bold ${themeClasses.text.primary}`}>Real-time Streaming</h3>
                  <div className="h-0.5 w-20 bg-[#00bfa6] mt-1 rounded-full"></div>
                </div>
              </div>

              {/* Connection Controls */}
              <div className="flex flex-wrap gap-4 mb-8">
                <button
                    onClick={startWebSocketStream}
                    disabled={isStreaming}
                    className={`px-8 py-4 ${themeClasses.primaryButton} disabled:opacity-50 disabled:cursor-not-allowed rounded-2xl font-bold transition-all duration-200 transform hover:scale-105 disabled:hover:scale-100 flex items-center gap-3 relative overflow-hidden group`}
                >
                  <div className="absolute inset-0 bg-gradient-to-r from-[#00bfa6]/20 to-[#00bfa6]/40 opacity-0 group-hover:opacity-100 transition-opacity duration-300"></div>
                  <div className="relative z-10 flex items-center gap-3">
                    <Play className="w-5 h-5" />
                    Start Stream
                  </div>
                </button>
                <button
                    onClick={stopWebSocketStream}
                    disabled={!isStreaming}
                    className={`px-8 py-4 bg-gradient-to-r from-red-600 to-red-700 hover:from-red-700 hover:to-red-800 disabled:opacity-50 disabled:cursor-not-allowed rounded-2xl font-bold text-white transition-all duration-200 transform hover:scale-105 disabled:hover:scale-100 shadow-lg hover:shadow-red-500/25 flex items-center gap-3`}
                >
                  <Square className="w-5 h-5" />
                  Stop Stream
                </button>
                <button
                    onClick={clearStreamMessages}
                    className={`px-8 py-4 ${themeClasses.secondaryButton} rounded-2xl font-bold transition-all duration-200 hover:scale-105`}
                >
                  Clear Messages
                </button>
              </div>

              {isStreaming && (streamMode === 'client' || streamMode === 'bidi') && (
                  <div className="mb-8">

                    <label className={`block text-sm font-semibold ${themeClasses.text.secondary} mb-4`}>Send Message</label>
                    <div className="flex gap-6">
                  <textarea
                      value={streamInput}
                      onChange={(e) => setStreamInput(e.target.value)}
                      placeholder='{"message": "Stream message"}'
                      rows={4}
                      className={`flex-1 p-5 ${themeClasses.input} rounded-2xl font-mono text-sm backdrop-blur-sm focus:ring-2 transition-all duration-200 resize-none`}
                  />
                      <div className="flex flex-col gap-4">
                        <button
                            onClick={sendStreamMessage}
                            className={`px-8 py-4 ${themeClasses.primaryButton} rounded-2xl font-bold transition-all duration-200 transform hover:scale-105 shadow-lg hover:shadow-[#00bfa6]/25 flex items-center gap-3`}
                        >
                          <Send className="w-5 h-5" />
                          Send
                        </button>
                        {streamMode === 'client' && (
                            <button
                                onClick={sendEndSignal}
                                className="px-8 py-4 bg-gradient-to-r from-orange-600 to-orange-700 hover:from-orange-700 hover:to-orange-800 rounded-2xl font-bold text-white transition-all duration-200 transform hover:scale-105 shadow-lg hover:shadow-orange-500/25"
                            >
                              End
                            </button>
                        )}
                      </div>
                    </div>
                  </div>
              )}

              {/* Server Stream Input */}
              {isStreaming && streamMode === 'server' && (
                  <div className="mb-8">
                    <label className={`block text-sm font-semibold ${themeClasses.text.secondary} mb-4`}>Initial Request</label>
                    <div className="flex gap-6">
                  <textarea
                      value={streamInput}
                      onChange={(e) => setStreamInput(e.target.value)}
                      placeholder='{"message": "Hello Stream"}'
                      rows={4}
                      className={`flex-1 p-5 ${themeClasses.input} rounded-2xl font-mono text-sm backdrop-blur-sm focus:ring-2 transition-all duration-200 resize-none`}
                  />
                      <button
                          onClick={sendStreamMessage}
                          className={`px-8 py-4 ${themeClasses.primaryButton} rounded-2xl font-bold transition-all duration-200 transform hover:scale-105 shadow-lg hover:shadow-[#00bfa6]/25 flex items-center gap-3 self-start`}
                      >
                        <Send className="w-5 h-5" />
                        Start
                      </button>
                    </div>
                  </div>
              )}
              {/* ────────────────────────────────────────────────────────────────
     METADATA + AUTH  •  ADVANCED PANEL
──────────────────────────────────────────────────────────────── */}
              <div className="mb-12 p-6 rounded-2xl border bg-white/60 dark:bg-black/30 border-gray-200 dark:border-gray-700 backdrop-blur-md shadow-sm">

                <h3 className="text-xl font-bold mb-6 text-gray-800 dark:text-gray-100">
                  Advanced Settings
                </h3>

                {/* ----- Metadata JSON ----- */}
                <div className="mb-8">
                  <label className="block text-sm font-semibold text-gray-700 dark:text-gray-300 mb-2">
                    Custom gRPC Metadata <span className="text-xs text-gray-500">(JSON)</span>
                  </label>
                  <textarea
                      rows={3}
                      value={metadataJson}
                      onChange={e => setMetadataJson(e.target.value)}
                      placeholder='{"x-api-key": "1234", "locale": "en-US"}'
                      className="w-full p-5 bg-white dark:bg-black/20 border border-gray-300 dark:border-gray-600 text-gray-900 dark:text-gray-100 placeholder-gray-400 dark:placeholder-gray-500 rounded-2xl font-mono text-sm focus:ring-2 focus:ring-emerald-500 focus:outline-none resize-none transition-all duration-200"
                  />
                </div>

                {/* ----- Authentication ----- */}
                <div>
                  <label className="block text-sm font-semibold text-gray-700 dark:text-gray-300 mb-2">
                    Authentication
                  </label>
                  <select
                      value={authType}
                      onChange={e => setAuthType(e.target.value)}
                      className="w-full p-4 bg-white dark:bg-black/20 border border-gray-300 dark:border-gray-600 text-gray-900 dark:text-gray-100 rounded-2xl text-sm focus:ring-2 focus:ring-emerald-500 focus:outline-none transition-all duration-200"
                  >
                    <option value="none">None</option>
                    <option value="bearer">Bearer / API Key</option>
                    <option value="basic">HTTP Basic</option>
                  </select>

                  {/* Bearer Token Input */}
                  {authType === 'bearer' && (
                      <input
                          type="text"
                          placeholder="Bearer / API token"
                          value={authToken}
                          onChange={e => setAuthToken(e.target.value)}
                          className="mt-4 w-full p-4 bg-white dark:bg-black/20 border border-gray-300 dark:border-gray-600 text-gray-900 dark:text-gray-100 placeholder-gray-400 dark:placeholder-gray-500 rounded-2xl font-mono text-sm focus:ring-2 focus:ring-emerald-500 focus:outline-none transition-all duration-200"
                      />
                  )}

                  {/* Basic Auth Inputs */}
                  {authType === 'basic' && (
                      <div className="mt-4 space-y-4">
                        <input
                            type="text"
                            placeholder="Username"
                            value={authUser}
                            onChange={e => setAuthUser(e.target.value)}
                            className="w-full p-4 bg-white dark:bg-black/20 border border-gray-300 dark:border-gray-600 text-gray-900 dark:text-gray-100 placeholder-gray-400 dark:placeholder-gray-500 rounded-2xl font-mono text-sm focus:ring-2 focus:ring-emerald-500 focus:outline-none transition-all duration-200"
                        />
                        <input
                            type="password"
                            placeholder="Password"
                            value={authPass}
                            onChange={e => setAuthPass(e.target.value)}
                            className="w-full p-4 bg-white dark:bg-black/20 border border-gray-300 dark:border-gray-600 text-gray-900 dark:text-gray-100 placeholder-gray-400 dark:placeholder-gray-500 rounded-2xl font-mono text-sm focus:ring-2 focus:ring-emerald-500 focus:outline-none transition-all duration-200"
                        />
                      </div>
                  )}
                </div>
              </div>

              {/* Stream Messages */}
              <div>
                <div className="flex items-center justify-between mb-6">
                  <label className={`text-2xl font-bold ${themeClasses.text.primary}`}>Message Stream</label>
                  <div className="flex items-center gap-4">
                    {isStreaming ? (
                        <div className="flex items-center gap-3 px-6 py-3 bg-gradient-to-r from-[#00bfa6]/20 via-[#00bfa6]/30 to-[#00bfa6]/20 border border-[#00bfa6]/40 rounded-full backdrop-blur-sm shadow-lg shadow-[#00bfa6]/20">
                          <div className="w-3 h-3 bg-[#00bfa6] rounded-full animate-pulse shadow-sm shadow-[#00bfa6]/50"></div>
                          <Wifi className="w-5 h-5 text-[#00bfa6] drop-shadow-sm" />
                          <span className="text-[#00bfa6] font-bold text-sm drop-shadow-sm">Connected</span>
                        </div>
                    ) : (
                        <div className={`flex items-center gap-3 px-6 py-3 ${darkMode ? 'bg-gray-500/20 border-gray-500/30' : 'bg-gray-200 border-gray-300'} border rounded-full backdrop-blur-sm`}>
                          <WifiOff className={`w-5 h-5 ${themeClasses.text.secondary}`} />
                          <span className={`${themeClasses.text.secondary} font-bold text-sm`}>Disconnected</span>
                        </div>
                    )}
                  </div>
                </div>


                <div className={`h-96 p-6 ${darkMode ? 'bg-black/30' : 'bg-gray-50/50'} border ${darkMode ? 'border-gray-700/50' : 'border-gray-200'} rounded-2xl overflow-y-auto backdrop-blur-sm custom-scrollbar`}>
                  {streamMessages.length === 0 ? (
                      <div className="h-full flex items-center justify-center">
                        <div className="text-center">
                          <MessageCircle className={`w-16 h-16 ${themeClasses.text.secondary} mx-auto mb-6 opacity-50`} />
                          <p className={`${themeClasses.text.secondary} text-xl font-semibold`}>No messages yet</p>
                          <p className={`${themeClasses.text.secondary} text-sm mt-2 opacity-75`}>Start a stream to see real-time messages</p>
                        </div>
                      </div>
                  ) : (
                      <div className="space-y-4">
                        {streamMessages.map((msg, index) => {
                          const styles = {
                            error: darkMode
                                ? 'bg-red-500/10 border-red-500/30 text-red-300'
                                : 'bg-red-50 border-red-200 text-red-700',
                            system: darkMode
                                ? 'bg-blue-500/10 border-blue-500/30 text-blue-300'
                                : 'bg-blue-50 border-blue-200 text-blue-700',
                            sent: darkMode
                                ? 'bg-[#00bfa6]/10 border-[#00bfa6]/30 text-[#00bfa6]'
                                : 'bg-[#00bfa6]/10 border-[#00bfa6]/30 text-[#00695c]',
                            response: darkMode
                                ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-300'
                                : 'bg-emerald-50 border-emerald-200 text-emerald-700'
                          };

                          return (
                              <div
                                  key={index}
                                  className={`p-5 rounded-2xl border backdrop-blur-sm transition-all duration-200 hover:scale-[1.01] ${styles[msg.type]}`}
                              >
                                <div className="flex justify-between items-center mb-3">
                            <span className="text-xs font-bold uppercase tracking-wider">
                              {msg.type === 'sent' ? 'Sent' : msg.type === 'response' ? 'Received' : msg.type}
                            </span>
                                  {msg.timestamp && (
                                      <span className="text-xs opacity-75 font-mono">{msg.timestamp}</span>
                                  )}
                                </div>
                                <pre className="text-sm whitespace-pre-wrap break-words font-mono leading-relaxed">
                            {msg.content}
                          </pre>
                              </div>
                          );
                        })}
                        <div ref={messagesEndRef} />
                      </div>
                  )}
                </div>
              </div>
            </div>
          </div>
        </div>

        <style jsx>{`
        .custom-scrollbar::-webkit-scrollbar {
          width: 12px;
        }
        .custom-scrollbar::-webkit-scrollbar-track {
          background: ${darkMode ? 'rgba(0, 0, 0, 0.4)' : 'rgba(0, 0, 0, 0.05)'};
          border-radius: 8px;
        }
        .custom-scrollbar::-webkit-scrollbar-thumb {
          background: linear-gradient(180deg, #00bfa6, #00897b);
          border-radius: 8px;
          box-shadow: inset 0 0 10px rgba(0, 191, 166, 0.3);
        }
        .custom-scrollbar::-webkit-scrollbar-thumb:hover {
          background: linear-gradient(180deg, #00897b, #00695c);
          box-shadow: inset 0 0 10px rgba(0, 191, 166, 0.5);
        }
      `}</style>
      </div>
  );
}