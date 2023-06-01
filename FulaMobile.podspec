Pod::Spec.new do |s|  
    s.name              = 'FulaMobile' # Name for your pod
    s.version           = '0.1.0'
    s.summary           = 'Go-fula for iOS'
    s.homepage          = 'https://github.com/functionland/go-fula'

    s.author            = { 'Functionland' => 'info@fx.land' }
    s.license = { :type => 'MIT', :file => 'LICENSE' }

    s.platform          = :ios
    # change the source location
    s.source            = { :http => "https://github.com/functionland/go-fula/releases/download/cocoapods-bundle/cocoapods-bundle.zip" } 
    s.source_files = "Headers/*.{h}"
    s.module_map = "Modules/module.modulemap"
    s.ios.deployment_target = '11.0'
    s.ios.vendored_libraries = 'libfula.a'
    s.static_framework = true
    s.user_target_xcconfig = { 'EXCLUDED_ARCHS[sdk=iphonesimulator*]' => 'arm64' }
    s.pod_target_xcconfig = { 'EXCLUDED_ARCHS[sdk=iphonesimulator*]' => 'arm64' }
end 