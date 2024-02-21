Pod::Spec.new do |s|  
    s.name              = 'Fula' # Name for your pod
    s.version           = "1.46.1"
    s.summary           = 'Go-fula for iOS'
    s.homepage          = 'https://github.com/functionland/go-fula'

    s.author            = { 'Functionland' => 'info@fx.land' }
    s.license = { :type => 'MIT', :file => 'LICENSE' }

    s.platform          = :ios
    # change the source location
    s.source            = { :http => "https://github.com/functionland/go-fula/releases/download/v1.46.1/cocoapods-bundle.zip" } 

    s.ios.deployment_target = '13.0'
    s.vendored_framework = 'Fula.xcframework'
    s.static_framework = true
    s.user_target_xcconfig = { 'EXCLUDED_ARCHS[sdk=iphonesimulator*]' => 'arm64' }
    s.pod_target_xcconfig = { 'EXCLUDED_ARCHS[sdk=iphonesimulator*]' => 'arm64' }
end 
